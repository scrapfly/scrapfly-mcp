package scrapflyprovider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ProgressNotifier struct {
	token    any
	session  *mcp.ServerSession
	total    float64
	progress float64
	started  sync.Once
	done     bool
}

func NewProgressNotifier(token any, session *mcp.ServerSession, total float64) *ProgressNotifier {
	return &ProgressNotifier{token: token, session: session, total: total, progress: 0.0}
}

func NewProgressNotifierFromRequest(req *mcp.CallToolRequest, total float64) (*ProgressNotifier, error) {
	token := req.Params.GetProgressToken()
	if token == nil {
		return nil, fmt.Errorf("progress token is nil")
	}
	return NewProgressNotifier(token, req.Session, total), nil
}

func makeProgressNotification(token any, progress, total float64, message string) *mcp.ProgressNotificationParams {
	params := &mcp.ProgressNotificationParams{
		ProgressToken: token,
		Progress:      progress,
	}
	if message != "" {
		params.Message = message
	}
	if total > 0 {
		params.Total = total
	}
	return params
}

func (p *ProgressNotifier) Start(ctx context.Context, message string) error {
	var err error
	p.started.Do(func() {
		err = p.session.NotifyProgress(ctx, makeProgressNotification(p.token, p.progress, p.total, message))
	})
	return err
}

func (p *ProgressNotifier) Progress(ctx context.Context, delta float64, message string) error {
	if p.done {
		return fmt.Errorf("progress already done")
	}
	if delta < 0 || delta == 0 {
		return fmt.Errorf("delta must be greater than 0")
	}
	p.progress += delta
	if p.total > 0 {
		if p.progress > p.total {
			p.progress = p.total
			p.done = true
		}
	}
	return p.session.NotifyProgress(ctx, makeProgressNotification(p.token, p.progress, p.total, message))
}

// routine to notify progress every 10 seconds until the scrape is complete or error via channel
// use specii channel to notify completion or error
func (p *ScrapflyToolProvider) progressRoutine(ctx context.Context, req *mcp.CallToolRequest, url string, stopChan <-chan struct{}) {
	progressNotifier, err := NewProgressNotifierFromRequest(req, 155.0) // 155 seconds is the max timeout for a scrape
	if err != nil {
		p.logger.Printf("Error creating progress notifier for Session: %s for url: %s, error: %v", req.Session.ID(), url, err)
		return
	}
	p.logger.Printf("Starting progress routine for Session: %s for url: %s, progress token: %v", req.Session.ID(), url, progressNotifier.token)
	err = progressNotifier.Start(ctx, "Started scraping request for url: "+url)
	if err != nil {
		p.logger.Printf("Error starting progress routine for Session: %s for url: %s, progress token: %v, error: %v", req.Session.ID(), url, progressNotifier.token, err)
		return
	}
	defer func() {
		p.logger.Printf("Stopping progress routine for Session: %s for url: %s, progress token: %v", req.Session.ID(), url, progressNotifier.token)
	}()
	for i := 10; i <= 155; i += 10 {
		select {
		case <-stopChan:
			return
		default:
			time.Sleep(10 * time.Second)
		}
		p.logger.Printf("Progressing progress routine for Session: %s for url: %s, progress token: %v, progress: %v", req.Session.ID(), url, progressNotifier.token, progressNotifier.progress)
		err = progressNotifier.Progress(ctx, 10.0, "Still scraping")
		if err != nil {
			// avoid "client is closing" as error
			if !strings.Contains(err.Error(), "client is closing") {
				p.logger.Printf("Error progressing progress routine for Session: %s for url: %s, progress token: %v, error: %v", req.Session.ID(), url, progressNotifier.token, err)
			}
			return
		}
	}
}
