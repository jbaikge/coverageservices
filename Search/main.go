package Search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/300brand/coverage"
	"github.com/300brand/coverage/social"
	"github.com/300brand/coverageservices/service"
	"github.com/300brand/coverageservices/types"
	"github.com/300brand/disgo"
	"github.com/300brand/go-toml-config"
	"github.com/300brand/logger"
	"labix.org/v2/mgo/bson"
	"net/http"
	"sync"
	"time"
)

type Service struct {
	client *disgo.Client
}

var (
	_ service.Service = &Service{}

	cfgSocialDelay = config.Duration("Search.socialdelay", time.Second)
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

	if _, err = http.Post(info.Notify.Done, "application/json", buf); err != nil {
		return
	}

	return
}

func (s *Service) Search(in *types.SearchQuery, out *types.SearchQueryResponse) (err error) {
	// Validation
	if in.Q == "" {
		return fmt.Errorf("Query cannot be empty")
	}
	if in.Dates.Start.After(in.Dates.End) {
		return fmt.Errorf("Invalid date range: %s > %s", in.Dates.Start, in.Dates.End)
	}

	// This is just silly, but most efficient way to calculate
	dates := []time.Time{}
	for st, t := in.Dates.Start.AddDate(0, 0, -1), in.Dates.End; t.After(st); t = t.AddDate(0, 0, -1) {
		dates = append(dates, t)
	}

	cs := coverage.NewSearch()
	cs.Notify = in.Notify
	cs.Q = in.Q
	cs.Label = in.Label
	cs.Dates = in.Dates
	cs.DaysLeft = len(dates)
	cs.PublicationIds = in.PublicationIds

	if err = s.client.Call("StorageWriter.NewSearch", cs, cs); err != nil {
		return
	}

	ds := types.DateSearch{
		Id:    cs.Id,
		Query: cs.Q,
	}
	var wg sync.WaitGroup
	for _, ds.Date = range dates {
		wg.Add(1)
		go func(ds types.DateSearch) {
			s.client.Call("StorageWriter.DateSearch", ds, disgo.Null)
			wg.Done()
		}(ds)
	}

	// If foregrounded, wait for everything to finish first
	if in.Foreground {
		logger.Trace.Printf("Waiting in foreground for DateSearches to finish")
		wg.Wait()
	}

	// Wait for all of the DateSearch calls to finish, then send the
	// notification of completeness
	go func(cs *coverage.Search) {
		wg.Wait()
		logger.Trace.Printf("Sending notifications to %s and %s", cs.Notify.Done, cs.Notify.Social)
		if cs.Notify.Done != "" {
			if err := s.client.Call("Search.SearchNotifyComplete", types.ObjectId{cs.Id}, disgo.Null); err != nil {
				logger.Error.Print(err)
			}
		}
		if cs.Notify.Social != "" {
			if err := s.client.Call("Search.Social", types.ObjectId{cs.Id}, disgo.Null); err != nil {
				logger.Error.Print(err)
			}
		}
		logger.Info.Printf("Search completed in %s", time.Since(cs.Start))
	}(cs)

	// Prepare information for the caller
	out.Id = cs.Id
	out.Start = cs.Start

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
				if _, err = http.Post(info.Notify.Social, "application/json", buf); err != nil {
					return
				}
			}(id)
			<-time.After(*cfgSocialDelay)
		}
	}(*info)

	return
}
