package Search

/*
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
*/
