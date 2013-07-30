package main

import (
	"git.300brand.com/coverage"
	"git.300brand.com/coverage/config"
	"git.300brand.com/coverage/skytypes"
	"git.300brand.com/coverage/storage/mongo"
	"github.com/skynetservices/skynet"
	"github.com/skynetservices/skynet/service"
	"labix.org/v2/mgo/bson"
	"log"
)

type Service struct{}

const ServiceName = "StorageWriter"

var (
	_ service.ServiceDelegate = &Service{}
	m *mongo.Mongo
)

// Funcs required for ServiceDelegate

func (s *Service) MethodCalled(m string) {}

func (s *Service) MethodCompleted(m string, d int64, err error) {}

func (s *Service) Registered(service *service.Service) {}

func (s *Service) Started(service *service.Service) {
	log.Printf("Connecting to MongoDB %s", config.Mongo.Host)
	m = mongo.New(config.Mongo.Host)
	m.Prefix = "A_"
	if err := m.Connect(); err != nil {
		log.Fatalf("Failed to connect to MongoDB: %s", err)
	}
	log.Println("Connected to MongoDB")
}

func (s *Service) Stopped(service *service.Service) {
	log.Println("Closing MongoDB connection")
	m.Close()
}

func (s *Service) Unregistered(service *service.Service) {}

// Service funcs

func (s *Service) AddSearchResults(ri *skynet.RequestInfo, in *skytypes.SearchResultSubset, out *skytypes.NullType) (err error) {
	results := make([]*coverage.SearchResult, len(in.ArticleIds))
	for i, a := range in.ArticleIds {
		results[i] = &coverage.SearchResult{
			SearchId:  in.Id,
			ArticleId: a,
		}
	}
	return m.AddSearchResults(in.Id, results)
}

func (s *Service) NewSearch(ri *skynet.RequestInfo, in *coverage.Search, out *coverage.Search) (err error) {
	*out = *in
	out.Id = bson.NewObjectId()
	return m.UpdateSearch(out)
}

func (s *Service) Article(ri *skynet.RequestInfo, in *coverage.Article, out *coverage.Article) (err error) {
	defer func() {
		*out = *in
	}()

	if err = m.AddURL(in.URL, in.ID); err != nil {
		log.Printf("Duplicate URL: %s", in.URL)
		return
	}
	if err = m.UpdateArticle(in); err != nil {
		return
	}
	go func(a *coverage.Article) {
		if err = m.AddKeywords(in); err != nil {
			log.Printf("Error saving keywords: %s", err)
		}
	}(in)
	return
}

func (s *Service) Feed(ri *skynet.RequestInfo, in *coverage.Feed, out *coverage.Feed) (err error) {
	defer func() {
		*out = *in
	}()

	if err = m.UpdateFeed(in); err != nil {
		return
	}
	return
}

// Main

func main() {
	log.SetFlags(log.Lshortfile | log.Lmicroseconds)
	log.SetPrefix(ServiceName + " ")

	sc, _ := skynet.GetServiceConfig()
	sc.Name = ServiceName
	sc.Region = "Storage"
	sc.Version = "1"

	s := service.CreateService(&Service{}, sc)
	defer s.Shutdown()

	s.Start(true).Wait()
}
