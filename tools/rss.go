package tools

import (
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"
)

type BlogPost struct {
	Title     string
	Link      string
	Published time.Time
}

type RSSReader struct {
	parser *gofeed.Parser
}

func NewRSSReader() *RSSReader {
	return &RSSReader{
		parser: gofeed.NewParser(),
	}
}

func (r *RSSReader) GetRecentPosts(feedURL string, limit int) ([]BlogPost, error) {
	if feedURL == "" {
		return nil, nil
	}

	feed, err := r.parser.ParseURL(feedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse feed: %v", err)
	}

	var posts []BlogPost
	for i, item := range feed.Items {
		if i >= limit {
			break
		}

		published := time.Now()
		if item.PublishedParsed != nil {
			published = *item.PublishedParsed
		}

		posts = append(posts, BlogPost{
			Title:     item.Title,
			Link:      item.Link,
			Published: published,
		})
	}

	return posts, nil
}
