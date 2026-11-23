package monitor

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/AobaIwaki123/taite-board/internal/raw"
	"github.com/AobaIwaki123/taite-board/internal/source/timetree"
	"github.com/AobaIwaki123/taite-board/internal/source/x"
)

// DomainRoutingFetcher dispatches FetchPost calls to the appropriate source adapter based on URL host.
type DomainRoutingFetcher struct {
	X        PostFetcher
	TimeTree PostFetcher
}

func newDefaultSourceFetcher() *DomainRoutingFetcher {
	return &DomainRoutingFetcher{
		X:        x.NewClient(15 * time.Second),
		TimeTree: timetree.NewClient(15 * time.Second),
	}
}

// FetchPost routes the URL to either X or TimeTree based on its domain.
func (f *DomainRoutingFetcher) FetchPost(ctx context.Context, rawURL string) (*raw.Post, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("有効な URL を入力してください: %s", rawURL)
	}

	host := strings.ToLower(u.Hostname())
	switch host {
	case "x.com", "www.x.com", "mobile.x.com", "twitter.com", "www.twitter.com", "mobile.twitter.com":
		if f.X == nil {
			return nil, fmt.Errorf("X source adapter is not configured")
		}
		return f.X.FetchPost(ctx, rawURL)
	case "timetreeapp.com", "www.timetreeapp.com", "timetr.ee", "www.timetr.ee":
		if f.TimeTree == nil {
			return nil, fmt.Errorf("TimeTree source adapter is not configured")
		}
		return f.TimeTree.FetchPost(ctx, rawURL)
	default:
		return nil, fmt.Errorf("未対応のドメインです (%s): X (x.com) または TimeTree (timetreeapp.com / timetr.ee) のイベントURLを指定してください", host)
	}
}
