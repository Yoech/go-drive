package search

import (
	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search"
	"go-drive/common/types"
	"go-drive/common/utils"
	"time"
)

type EntrySearcher struct {
	index bleve.Index
}

func NewEntrySearcher(indexPath string) (*EntrySearcher, error) {
	var index bleve.Index
	var e error
	if exists, _ := utils.FileExists(indexPath); exists {
		index, e = bleve.Open(indexPath)
	} else {
		index, e = bleve.New(indexPath, createMapping())
	}
	if e != nil {
		return nil, e
	}
	return &EntrySearcher{index: index}, nil
}

func createMapping() mapping.IndexMapping {
	m := bleve.NewIndexMapping()
	entryMapping := bleve.NewDocumentMapping()

	entryMapping.AddFieldMappingsAt("Path", bleve.NewTextFieldMapping())
	entryMapping.AddFieldMappingsAt("Name", bleve.NewTextFieldMapping())
	entryMapping.AddFieldMappingsAt("Ext", bleve.NewKeywordFieldMapping())
	entryMapping.AddFieldMappingsAt("Type", bleve.NewKeywordFieldMapping())
	entryMapping.AddFieldMappingsAt("Size", bleve.NewNumericFieldMapping())
	entryMapping.AddFieldMappingsAt("ModifiedAt", bleve.NewNumericFieldMapping())

	m.DefaultMapping = entryMapping
	return m
}

func (s *EntrySearcher) Search(path string, query string, from, size int) ([]types.EntrySearchResultItem, error) {
	if path != "" {
		path += "/"
	}

	pq := bleve.NewPrefixQuery(path)
	pq.SetField("path")

	bqn := bleve.NewBooleanQuery()
	// exclude this query from highlights
	bqn.AddMustNot(pq)

	qq := bleve.NewQueryStringQuery(query)

	bq := bleve.NewBooleanQuery()
	bq.AddMustNot(bqn)
	bq.AddMust(qq)

	sr := bleve.NewSearchRequestOptions(bq, size, from, false)

	sr.Fields = []string{"*"}

	sr.Highlight = bleve.NewHighlight()
	sr.Highlight.AddField("path")
	sr.Highlight.AddField("name")

	result, e := s.index.Search(sr)
	if e != nil {
		return nil, e
	}

	return mapSearchResultItem(result.Hits), nil
}

// Index add or update an entry to the index
func (s *EntrySearcher) Index(entry types.EntrySearchItem) error {
	return s.index.Index(entry.Path, entry)
}

// Delete remove an entry from the index
func (s *EntrySearcher) Delete(path string) error {
	return s.index.Delete(path)
}

// DeleteDir remove all entries in the dir from the index
func (s *EntrySearcher) DeleteDir(ctx types.TaskCtx, dirPath string) error {
	ctx.Total(1, false)
	total := uint64(0)
	ps := bleve.NewPrefixQuery(dirPath + "/")
	for {
		if e := ctx.Err(); e != nil {
			return e
		}
		req := bleve.NewSearchRequestOptions(ps, 1000, 0, false)
		r, e := s.index.Search(req)
		if e != nil {
			return e
		}
		if total == 0 {
			total = r.Total
			ctx.Total(int64(total), false)
		}
		if total == 0 {
			break
		}
		for _, hit := range r.Hits {
			if e := ctx.Err(); e != nil {
				return e
			}
			e := s.Delete(hit.ID)
			if e != nil {
				return e
			}
			ctx.Progress(1, false)
		}
	}
	e := s.Delete(dirPath)
	if e != nil {
		return e
	}
	ctx.Progress(1, false)
	return nil
}

func (s *EntrySearcher) Stats() (SearcherStats, error) {
	docs, e := s.index.DocCount()
	if e != nil {
		return SearcherStats{}, e
	}
	stats := s.index.StatsMap()
	return SearcherStats{
		Total:      docs,
		Searches:   stats["searches"].(uint64),
		SearchTime: stats["search_time"].(uint64),
	}, nil
}

func (s *EntrySearcher) Dispose() error {
	return s.index.Close()
}

func mapSearchResultItem(hits search.DocumentMatchCollection) []types.EntrySearchResultItem {
	items := make([]types.EntrySearchResultItem, 0, len(hits))
	for _, hit := range hits {
		modTime, _ := time.Parse(time.RFC3339, hit.Fields["modifiedAt"].(string))
		esi := types.EntrySearchItem{
			Path:       hit.Fields["path"].(string),
			Name:       hit.Fields["name"].(string),
			Ext:        hit.Fields["ext"].(string),
			Type:       types.EntryType(hit.Fields["type"].(string)),
			Size:       int64(hit.Fields["size"].(float64)),
			ModifiedAt: modTime,
		}
		highlights := make(map[string][]string, len(hit.Locations))
		for k, v := range hit.Locations {
			segments := make([]string, 0, len(v))
			for seg := range v {
				segments = append(segments, seg)
			}
			highlights[k] = segments
		}
		item := types.EntrySearchResultItem{
			Entry:      esi,
			Highlights: highlights,
		}

		items = append(items, item)
	}
	return items
}

type SearcherStats struct {
	Total      uint64
	Searches   uint64
	SearchTime uint64
}