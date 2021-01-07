package blacklister

import (
	"fmt"
	"github.com/creekorful/trandoshan/internal/cache"
	configapi "github.com/creekorful/trandoshan/internal/configapi/client"
	"github.com/creekorful/trandoshan/internal/event"
	chttp "github.com/creekorful/trandoshan/internal/http"
	"github.com/creekorful/trandoshan/internal/process"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
	"net/http"
	"net/url"
)

var errAlreadyBlacklisted = fmt.Errorf("hostname is already blacklisted")

// State represent the application state
type State struct {
	configClient  configapi.Client
	hostnameCache cache.Cache
	httpClient    chttp.Client
}

// Name return the process name
func (state *State) Name() string {
	return "blacklister"
}

// Features return the process features
func (state *State) Features() []process.Feature {
	return []process.Feature{process.EventFeature, process.ConfigFeature, process.CacheFeature, process.CrawlingFeature}
}

// CustomFlags return process custom flags
func (state *State) CustomFlags() []cli.Flag {
	return []cli.Flag{}
}

// Initialize the process
func (state *State) Initialize(provider process.Provider) error {
	hostnameCache, err := provider.Cache("down-hostname")
	if err != nil {
		return err
	}
	state.hostnameCache = hostnameCache

	configClient, err := provider.ConfigClient([]string{configapi.ForbiddenHostnamesKey, configapi.BlackListThresholdKey})
	if err != nil {
		return err
	}
	state.configClient = configClient

	httpClient, err := provider.HTTPClient()
	if err != nil {
		return err
	}
	state.httpClient = httpClient

	return nil
}

// Subscribers return the process subscribers
func (state *State) Subscribers() []process.SubscriberDef {
	return []process.SubscriberDef{
		{Exchange: event.TimeoutURLExchange, Queue: "blacklistingQueue", Handler: state.handleTimeoutURLEvent},
	}
}

// HTTPHandler returns the HTTP API the process expose
func (state *State) HTTPHandler() http.Handler {
	return nil
}

func (state *State) handleTimeoutURLEvent(subscriber event.Subscriber, msg event.RawMessage) error {
	var evt event.TimeoutURLEvent
	if err := subscriber.Read(&msg, &evt); err != nil {
		return err
	}

	u, err := url.Parse(evt.URL)
	if err != nil {
		return err
	}

	// Make sure hostname is not already 'blacklisted'
	forbiddenHostnames, err := state.configClient.GetForbiddenHostnames()
	if err != nil {
		return err
	}

	// prevent duplicates
	found := false
	for _, hostname := range forbiddenHostnames {
		if hostname.Hostname == u.Hostname() {
			found = true
			break
		}
	}

	if found {
		return fmt.Errorf("%s %w", u.Hostname(), errAlreadyBlacklisted)
	}

	// Check by ourselves if the hostname doesn't respond
	_, err = state.httpClient.Get(fmt.Sprintf("%s://%s", u.Scheme, u.Host))
	if err == nil || err != chttp.ErrTimeout {
		return nil
	}

	log.Debug().
		Str("hostname", u.Hostname()).
		Msg("Timeout confirmed")

	threshold, err := state.configClient.GetBlackListThreshold()
	if err != nil {
		return err
	}

	cacheKey := u.Hostname()
	count, err := state.hostnameCache.GetInt64(cacheKey)
	if err != nil {
		return err
	}
	count++

	if count >= threshold.Threshold {
		forbiddenHostnames, err := state.configClient.GetForbiddenHostnames()
		if err != nil {
			return err
		}

		// prevent duplicates
		found := false
		for _, hostname := range forbiddenHostnames {
			if hostname.Hostname == u.Hostname() {
				found = true
				break
			}
		}

		if found {
			log.Trace().Str("hostname", u.Hostname()).Msg("Skipping duplicate hostname")
		} else {
			log.Info().
				Str("hostname", u.Hostname()).
				Int64("count", count).
				Msg("Blacklisting hostname")

			forbiddenHostnames = append(forbiddenHostnames, configapi.ForbiddenHostname{Hostname: u.Hostname()})
			if err := state.configClient.Set(configapi.ForbiddenHostnamesKey, forbiddenHostnames); err != nil {
				return err
			}
		}
	}

	// Update count
	if err := state.hostnameCache.SetInt64(cacheKey, count, cache.NoTTL); err != nil {
		return err
	}

	return nil
}
