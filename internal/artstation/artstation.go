package artstation

import (
	"encoding/xml"
	"io"
	"net/http"
	"strings"
	"time"

	"charm.land/log/v2"
)

type item struct {
	Title       string  `xml:"title"`
	Description string  `xml:"description"`
	Link        string  `xml:"link"`
	PublishDate rssTime `xml:"pubDate"`
}

type rssTime struct {
	time.Time
}

func (t *rssTime) UnmarshalText(b []byte) error {
	parsed, err := time.Parse(time.RFC1123Z, string(b))
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

type projectList struct {
	Items []item `xml:"channel>item"`
}

const scrapeInterval = 10 * time.Minute
const rssURL = "https://www.artstation.com/mistromy.rss"

var client = http.Client{Timeout: time.Second * 5}

func Init() {
	log.Info("Loading Artstation Module", "Scrape Interval", scrapeInterval)
	list := scrape()
	log.Info("Titles found", "titles", returnTitles(list))
	ticker := time.NewTicker(scrapeInterval)
	defer ticker.Stop()
	for range ticker.C {
		scrape()
	}
}

func scrape() projectList {
	request, err := http.NewRequest(http.MethodGet, rssURL, nil)
	if err != nil {
		log.Error("MakeRequest", "error", err)
		return projectList{}
	}
	request.Header.Set("Accept", "application/rss+xml")
	request.Header.Add("User-Agent", "MistAPI/1.0 (+https://mista.tech)")
	response, err := client.Do(request)
	if err != nil {
		log.Error("Execute Requst", "error", err)
		return projectList{}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		log.Warn("Fetch RSS", "StatusCode", response.StatusCode)
		return projectList{}
	}
	var list projectList
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Error("Read Response Body", "error", err)
		return projectList{}
	}
	err = xml.Unmarshal(body, &list)
	if err != nil {
		log.Error("Unmarshal XML", "error", err)
		return projectList{}
	}
	return list
}

func returnTitles(list projectList) string {
	var b strings.Builder
	for _, v := range list.Items {
		b.WriteString(strings.TrimSuffix(v.Title, " by Mist") + "\n")
	}
	return b.String()
}
