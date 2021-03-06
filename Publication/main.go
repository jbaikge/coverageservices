package Publication

import (
	"github.com/300brand/coverage"
	"github.com/300brand/coverageservices/service"
	"github.com/300brand/coverageservices/types"
	"github.com/300brand/disgo"
	"labix.org/v2/mgo/bson"
	"net/url"
)

type PubsArr []types.Pub

type Service struct {
	client *disgo.Client
}

var _ service.Service = new(Service)

func init() {
	service.Register("Publication", new(Service))
}

// Funcs required for Service

func (s *Service) Start(client *disgo.Client) (err error) {
	s.client = client
	return
}

// Service funcs

func (s *Service) Add(in *types.Pub, out *coverage.Publication) (err error) {
	p := coverage.NewPublication()
	p.Title = in.Title
	p.NumReaders = in.Readership
	if _, err = url.Parse(in.URL); err != nil {
		return
	}
	p.URL = in.URL
	feeds := make([]*coverage.Feed, len(in.Feeds))
	for i, feedUrl := range in.Feeds {
		feeds[i] = coverage.NewFeed()
		feeds[i].PublicationId = p.ID
		feeds[i].URL = feedUrl
		p.NumFeeds++
	}
	if err = s.client.Call("StorageWriter.Publication", p, disgo.Null); err != nil {
		return
	}
	for _, f := range feeds {
		if err = s.client.Call("StorageWriter.Feed", f, disgo.Null); err != nil {
			continue
		}
	}
	*out = *p
	return
}

func (s *Service) AddAll(in *PubsArr, out *disgo.NullType) (err error) {
	p := new(coverage.Publication)
	for _, pub := range []types.Pub(*in) {
		if err = s.Add(&pub, p); err != nil {
			return
		}
	}
	return
}

func (s *Service) View(in *types.ViewPubQuery, out *types.ViewPub) (err error) {
	pubId := &types.ObjectId{Id: in.Publication}
	if err = s.client.Call("StorageReader.Publication", pubId, &out.Publication); err != nil {
		return
	}

	if in.Feeds.Query == nil {
		in.Feeds.Query = make(bson.M)
	}
	in.Feeds.Query["publicationid"] = in.Publication
	if err = s.client.Call("StorageReader.Feeds", in.Feeds, &out.Feeds); err != nil {
		return
	}

	if in.Articles.Query == nil {
		in.Articles.Query = make(bson.M)
	}
	in.Articles.Query["publicationid"] = in.Publication
	if err = s.client.Call("StorageReader.Articles", in.Articles, &out.Articles); err != nil {
		return
	}
	return
}
