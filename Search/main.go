package Search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/300brand/coverage"
	"github.com/300brand/coverage/article/lexer"
	"github.com/300brand/coverage/social"
	"github.com/300brand/coverageservices/service"
	"github.com/300brand/coverageservices/types"
	"github.com/300brand/disgo"
	"github.com/300brand/go-toml-config"
	"github.com/300brand/logger"
	"github.com/300brand/mongosearch"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"net/http"
	"strings"
	"time"
)

type Service struct {
	client *disgo.Client
}

var (
	_ service.Service = &Service{}

	cfgSocialDelay = config.Duration("Search.socialdelay", time.Second)
	cfgMongoServer = config.String("Search.mongodb", "127.0.0.1:27017")
)

func init() {
	service.Register("Search", new(Service))
}

// Funcs required for Service

func (s *Service) Start(client *disgo.Client) (err error) {
	s.client = client
	return
}

// Service funcs

func (s *Service) SearchNotifyComplete(in *types.ObjectId, out *disgo.NullType) (err error) {
	info := new(coverage.Search)
	if err = s.client.Call("StorageReader.Search", in, info); err != nil {
		return
	}

	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	if err = enc.Encode(info); err != nil {
		return
	}

	resp, err := http.Post(info.Notify.Done, "application/json", buf)
	if err != nil {
		return
	}
	resp.Body.Close()

	return
}

func (s *Service) Search(in *types.SearchQuery, out *types.SearchQueryResponse) (err error) {
	id := bson.NewObjectId()
	start := time.Now()

	{ // Fill in legacy search document for export later (TODO Remove later?)
		cs := coverage.NewSearch()
		cs.Id = id
		cs.Notify = in.Notify
		cs.Q = in.Q
		cs.Label = in.Label
		cs.Dates = in.Dates
		cs.PublicationIds = in.PublicationIds
		if err = s.client.Call("StorageWriter.NewSearch", cs, cs); err != nil {
			return
		}
	}

	search, err := mongosearch.New(*cfgMongoServer, "300brand_Articles.Articles", "300brand_Search.Results")
	if err != nil {
		return
	}

	keywordFunc := func(s string) (out interface{}, isArray bool, err error) {
		isArray = true
		out = lexer.Keywords([]byte(s))
		return
	}
	search.SetCaseSensitive(in.CaseSensitive)
	search.SetAll("text.words.all")
	search.SetKeyword("text.words.keywords", keywordFunc, "keywords")
	search.SetPubdate("pubdate.date", mongosearch.ConvertDateInt, "published")
	search.SetPubid("publicationid", mongosearch.ConvertBsonId, "publicationid")

	// This is just silly, but most efficient way to calculate
	dates := []time.Time{}
	for st, t := in.Dates.Start.AddDate(0, 0, -1), in.Dates.End; t.After(st); t = t.AddDate(0, 0, -1) {
		dates = append(dates, t)
	}
	// Cast dates to string and proper format
	queryDates := make([]string, len(dates))
	for i := range dates {
		queryDates[i] = fmt.Sprintf("'%s'", dates[i].Format(mongosearch.TimeLayout))
	}

	queryIn := ""
	switch in.Version {
	case 0, 1:
		queryIn = queryV1toV2(in.Q)
	case 2:
		// Can't decide if the date range should be expected in the input?
		queryIn = in.Q
	default:
		return fmt.Errorf("Invalid version: %d", in.Version)
	}

	query := fmt.Sprintf(
		"published:(%s) AND keywords:(%s)",
		strings.Join(queryDates, " OR "),
		queryIn,
	)

	if len(in.PublicationIds) > 0 {
		ids := make([]string, len(in.PublicationIds))
		for i, id := range in.PublicationIds {
			ids[i] = id.Hex()
		}
		query += fmt.Sprintf(" AND publicationid:(%s)", strings.Join(ids, " OR "))
	}

	logger.Warn.Printf("Search.Search: Sending %s", query)

	doSearch := func() {
		if err := search.SearchInto(query, id); err != nil {
			logger.Error.Printf("SearchInto: [%s] [%s] - %s", id.Hex(), query, err)
			return
		}

		{ // Update search completion; transfer IDs (TODO Remove later?)
			session, err := mgo.Dial(*cfgMongoServer)
			if err != nil {
				logger.Error.Printf("Error connecting to MongoDB: %s", err)
				return
			}
			defer session.Close()

			ids := []struct {
				Id bson.ObjectId `bson:"_id"`
			}{}
			db := session.DB("300brand_Search")
			if err := db.C("Results_" + id.Hex()).Find(nil).Select(bson.M{"_id": 1}).All(&ids); err != nil {
				logger.Error.Printf("Error retrieving all IDs from Results_%s: %s", id.Hex(), err)
				return
			}

			articleids := make([]bson.ObjectId, len(ids))
			for i := range ids {
				articleids[i] = ids[i].Id
			}
			if err := db.C("Search").UpdateId(id, bson.M{
				"$set": bson.M{
					"completed": time.Now(),
					"articles":  articleids,
					"results":   len(articleids),
				},
			}); err != nil {
				logger.Error.Printf("Error updating search record [%s]: %s", id.Hex(), err)
				return
			}
		}

		logger.Trace.Printf("Sending notifications to %s and %s", in.Notify.Done, in.Notify.Social)
		if in.Notify.Done != "" {
			if err := s.client.Call("Search.SearchNotifyComplete", types.ObjectId{id}, disgo.Null); err != nil {
				logger.Error.Print(err)
			}
		}
		if in.Notify.Social != "" {
			if err := s.client.Call("Search.Social", types.ObjectId{id}, disgo.Null); err != nil {
				logger.Error.Print(err)
			}
		}
		logger.Info.Printf("Search completed in %s", time.Since(start))
	}

	if in.Foreground {
		doSearch()
	} else {
		go doSearch()
	}

	out.Id = id
	out.Start = start

	return
}

func (s *Service) Social(in *types.ObjectId, out *disgo.NullType) (err error) {
	info := &coverage.Search{}
	if err = s.client.Call("StorageReader.Search", in, info); err != nil {
		return
	}

	go func(info coverage.Search) {
		for _, id := range info.Articles {
			go func(id bson.ObjectId) {
				// Get article from DB
				logger.Debug.Printf("Getting %s from DB", id.Hex())
				a := &coverage.Article{}
				if err := s.client.Call("StorageReader.Article", types.ObjectId{id}, a); err != nil {
					logger.Error.Print(err)
					return
				}
				// Get stats
				logger.Debug.Printf("Calling Social.Article for %s", id.Hex())
				var socialStats social.Stats
				if err := s.client.Call("Social.Article", a, &socialStats); err != nil {
					logger.Error.Print(err)
					return
				}
				// Send stats to frontend
				stats := struct {
					ArticleId, SearchId bson.ObjectId
					Stats               social.Stats
				}{id, info.Id, socialStats}

				buf := new(bytes.Buffer)
				enc := json.NewEncoder(buf)
				if err = enc.Encode(stats); err != nil {
					return
				}
				logger.Debug.Printf("Sending %+v to %s", stats, info.Notify.Social)

				resp, err := http.Post(info.Notify.Social, "application/json", buf)
				if err != nil {
					logger.Error.Printf("Error sending notifiation: %s", err)
					return
				}
				resp.Body.Close()
			}(id)
			<-time.After(*cfgSocialDelay)
		}
	}(*info)

	return
}
