package StorageReader

import (
	"git.300brand.com/coverage"
	"git.300brand.com/coverage/storage/mongo"
	"git.300brand.com/coverageservices/service"
	"git.300brand.com/coverageservices/types"
	"github.com/jbaikge/disgo"
	"github.com/jbaikge/logger"
)

type StorageReader struct {
	client *disgo.Client
	config cfgStorageReader
	m      *mongo.Mongo
}

type cfgStorageReader struct {
	// Prefix for database names (used when running production and testing in
	// same environment)
	Prefix string
	// Addresses of MongoDB, see labix.org/mgo for format details
	MongoDB string
}

var _ service.Service = new(StorageReader)

func init() {
	service.Register("StorageReader", &StorageReader{
		config: cfgStorageReader{
			Prefix:  "A_",
			MongoDB: "127.0.0.1",
		},
	})
}

// Funcs required for Service

func (s *StorageReader) ConfigOptions() interface{} {
	return &s.config
}

func (s *StorageReader) Start(client *disgo.Client) (err error) {
	s.client = client

	logger.Debug.Printf("StorageReader: Connecting to MongoDB %s", s.config.MongoDB)
	s.m = mongo.New(s.config.MongoDB)
	s.m.Prefix = s.config.Prefix
	if err = s.m.Connect(); err != nil {
		logger.Error.Printf("StorageReader: Failed to connect to MongoDB: %s", err)
		return
	}
	logger.Debug.Println("StorageReader: Connected to MongoDB")
	return
}

// Service funcs

func (s *StorageReader) Article(in *types.ObjectId, out *coverage.Article) error {
	return s.m.GetArticle(in.Id, out)
}

func (s *StorageReader) Feed(in *types.ObjectId, out *coverage.Feed) error {
	return s.m.GetFeed(in.Id, out)
}

func (s *StorageReader) OldestFeed(in *types.ObjectIds, out *coverage.Feed) error {
	return s.m.GetOldestFeed(in.Ids, out)
}

func (s *StorageReader) Publication(in *types.ObjectId, out *coverage.Publication) error {
	return s.m.GetPublication(in.Id, out)
}

func (s *StorageReader) Publications(in *types.MultiQuery, out *types.MultiPubs) (err error) {
	if out.Total, err = s.m.C.Publications.Find(in.Query).Count(); err != nil {
		return
	}
	out.Query = *in
	out.Publications = make([]*coverage.Publication, 0, in.Limit)
	return s.m.GetPublications(in.Query, in.Sort, in.Skip, in.Limit, &out.Publications)
}

func (s *StorageReader) Search(in *types.ObjectId, out *coverage.Search) error {
	return s.m.GetSearch(in.Id, out)
}
