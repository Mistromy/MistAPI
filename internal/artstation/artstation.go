package artstation

import (
	"encoding/xml"
	"io"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"charm.land/log/v2"
	"github.com/mistromy/MistAPI/internal/projects"
)

type item struct {
	Title       string  `xml:"title"`
	Description string  `xml:"description"`
	Link        string  `xml:"link"`
	PublishDate rssTime `xml:"pubDate"`
	Content     string  `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
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
	var dbWork []projects.Work
	for _, work := range slices.Backward(list.Items) {
		dbWork = append(dbWork, projects.Work{Source: projects.SourceTypeArtstation,
			SourceID:    path.Base(work.Link),
			Title:       work.Title,
			Description: work.Description,
			SourceURL:   work.Link,
			PublishedAt: work.PublishDate.Time,
			Images:      nil,
		})
	}
	err := projects.AddArtwork(dbWork)
	if err != nil {
		log.Warn("Add Artwork to DB", "error", err)
	}

	log.Debug("Number of artworks", "count", list.count())
	log.Debug(list.Items[1].images())
	//log.Debug("Titles found", "titles", list.titles())

	ticker := time.NewTicker(scrapeInterval)
	defer ticker.Stop()
	for range ticker.C {
		scrape()
	}
}

// scrape Returns a projectList struct populated by the rss content from my Artstation
//
//	type item struct {
//	Title       string  `xml:"title"`
//	Description string  `xml:"description"`
//	Link        string  `xml:"link"`
//	PublishDate rssTime `xml:"pubDate"`
//	}
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
		log.Error("Execute Request", "error", err)
		return projectList{}
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Error("Body Close", "error", err)
			return
		}
	}(response.Body)
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

// titles method gives a string of all titles from the rss feed (testing)
func (list projectList) titles() string {
	var b strings.Builder
	for _, v := range list.Items {
		b.WriteString(strings.TrimSuffix(v.Title, " by Mist") + "\n")
	}
	return b.String()
}

// imgSrc is the regex pattern to extract image links from the rss feed's "<content:encoded>" entry
var imgSrc = regexp.MustCompile(`<img src="([^"]+)`)

// images method returns []string of all image URLs in the specific item using regex
func (item item) images() (images []string) {
	for _, m := range imgSrc.FindAllStringSubmatch(item.Content, -1) {
		images = append(images, m[1])
	}
	return images
}

func (list projectList) count() int {
	return len(list.Items)
}
