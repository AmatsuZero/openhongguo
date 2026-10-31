package sdk_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/beilunyang/openhongguo/adapters/httptransport"
	"github.com/beilunyang/openhongguo/adapters/memstore"
	"github.com/beilunyang/openhongguo/sdk"
)

const comicID = "7111944514343799844"

func comicLoader() map[string]any {
	return map[string]any{
		"book_id": comicID, "title": "Fixture Comic", "can_read_episode_cnt": 1,
		"next_vid": "chapter2", "content": comicContent([]any{comicChapter("chapter1")}),
	}
}

func comicChapter(id string) map[string]any {
	return map[string]any{"item_id": id, "pic_list": []any{
		map[string]any{"pic_url": "https://p9-novel-sign.byteimg.com/novel-pic/one.png?x-signature=fixture", "width": 1500, "height": 2666},
		map[string]any{"pic_url": "https://p9-novel-sign.byteimg.com/novel-pic/two.png?x-signature=fixture", "width": 1200, "height": 2000},
	}}
}

func comicContent(chapters []any) string {
	b, _ := json.Marshal(map[string]any{"chapter_info_list": chapters})
	return string(b)
}

func comicHTML(loader map[string]any) string {
	b, _ := json.Marshal(map[string]any{"loaderData": map[string]any{"comic_(id)/page": loader}, "errors": nil})
	return "<html><script>window._ROUTER_DATA = " + string(b) + ";</script></html>"
}

func comicClient(t *testing.T, handler http.HandlerFunc) *sdk.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := sdk.NewClient(sdk.Options{Transport: httptransport.New(), ProfileStore: memstore.New(), WebOrigin: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func htmlHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, body)
	}
}

func TestGetComicReaderPublicImages(t *testing.T) {
	calls := 0
	c := comicClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/comic/"+comicID || r.URL.RawQuery != "" {
			t.Errorf("request = %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("public request sent credentials")
		}
		htmlHandler(comicHTML(comicLoader()))(w, r)
	})
	r, err := c.GetComicReader(context.Background(), comicID)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || r.BookID != comicID || r.Title != "Fixture Comic" || r.CanReadEpisodeCount != 1 || r.NextEpisodeID != "chapter2" {
		t.Fatalf("result = %+v; calls=%d", r, calls)
	}
	if r.SourceURL != "https://hongguoduanju.com/comic/"+comicID {
		t.Fatalf("source = %s", r.SourceURL)
	}
	if len(r.Chapters) != 1 || r.Chapters[0].ID != "chapter1" || r.Chapters[0].Number != 1 || len(r.Chapters[0].Pages) != 2 {
		t.Fatalf("chapters = %+v", r.Chapters)
	}
	pages := r.Chapters[0].Pages
	if pages[0].Width != 1500 || pages[0].Height != 2666 || pages[1].Width != 1200 || !strings.HasSuffix(pages[1].URL, "two.png?x-signature=fixture") {
		t.Fatalf("pages = %+v", pages)
	}
	if pages[0].ID == "" || pages[0].ID == pages[1].ID {
		t.Fatalf("page IDs = %+v", pages)
	}
}

func TestGetComicReaderZeroAndMultipleChapters(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			loader := comicLoader()
			loader["can_read_episode_cnt"] = count
			loader["content"] = comicContent([]any{comicChapter("chapter1"), comicChapter("chapter2")})
			if count == 0 {
				loader["content"] = ""
			}
			r, err := comicClient(t, htmlHandler(comicHTML(loader))).GetComicReader(context.Background(), comicID)
			if err != nil {
				t.Fatal(err)
			}
			if r.CanReadEpisodeCount != count || len(r.Chapters) != count || r.Chapters == nil {
				t.Fatalf("result = %+v", r)
			}
			if count == 2 && (r.Chapters[1].ID != "chapter2" || r.Chapters[1].Number != 2) {
				t.Fatalf("chapters = %+v", r.Chapters)
			}
		})
	}
}

func TestGetComicReaderRejectsMalformedPublicContent(t *testing.T) {
	cases := map[string]func(map[string]any){
		"wrong book":       func(l map[string]any) { l["book_id"] = "999999" },
		"missing count":    func(l map[string]any) { delete(l, "can_read_episode_cnt") },
		"negative count":   func(l map[string]any) { l["can_read_episode_cnt"] = -1 },
		"fractional count": func(l map[string]any) { l["can_read_episode_cnt"] = 1.5 },
		"missing body":     func(l map[string]any) { l["content"] = "" },
		"invalid JSON":     func(l map[string]any) { l["content"] = "{" },
		"wrong body type":  func(l map[string]any) { l["content"] = `"text"` },
		"no chapters":      func(l map[string]any) { l["content"] = comicContent(nil) },
		"empty chapter":    func(l map[string]any) { l["content"] = comicContent([]any{map[string]any{}}) },
		"partial bad image": func(l map[string]any) {
			c := comicChapter("chapter1")
			c["pic_list"] = append(c["pic_list"].([]any), nil)
			l["content"] = comicContent([]any{c})
		},
		"duplicate chapter": func(l map[string]any) {
			l["can_read_episode_cnt"] = 2
			l["content"] = comicContent([]any{comicChapter("c"), comicChapter("c")})
		},
		"short body": func(l map[string]any) { l["can_read_episode_cnt"] = 2 },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			l := comicLoader()
			change(l)
			if r, err := comicClient(t, htmlHandler(comicHTML(l))).GetComicReader(context.Background(), comicID); err == nil {
				t.Fatalf("accepted malformed result: %+v", r)
			}
		})
	}
}

func TestGetComicReaderRejectsUnsafePages(t *testing.T) {
	for _, address := range []string{
		"http://p9-novel-sign.byteimg.com/novel-pic/a.png", "https://evil.example/a.png",
		"https://p9-novel-sign.byteimg.com.evil.example/novel-pic/a.png", "https://user@p9-novel-sign.byteimg.com/novel-pic/a.png",
		"https://p9-novel-sign.byteimg.com:8443/novel-pic/a.png", "https://p9-novel-sign.byteimg.com/other/a.png",
	} {
		t.Run(address, func(t *testing.T) {
			l := comicLoader()
			ch := comicChapter("c")
			ch["pic_list"].([]any)[0].(map[string]any)["pic_url"] = address
			l["content"] = comicContent([]any{ch})
			if _, err := comicClient(t, htmlHandler(comicHTML(l))).GetComicReader(context.Background(), comicID); err == nil {
				t.Fatal("accepted unsafe image")
			}
		})
	}
	for _, dimension := range []any{0, -1, 20001, 1.5, "bad", nil} {
		t.Run(fmt.Sprint(dimension), func(t *testing.T) {
			l := comicLoader()
			ch := comicChapter("c")
			ch["pic_list"].([]any)[0].(map[string]any)["width"] = dimension
			l["content"] = comicContent([]any{ch})
			if _, err := comicClient(t, htmlHandler(comicHTML(l))).GetComicReader(context.Background(), comicID); err == nil {
				t.Fatal("accepted invalid dimensions")
			}
		})
	}
}

func TestGetComicReaderRequestBounds(t *testing.T) {
	for _, id := range []string{"not-a-book", "123", "12345678901234567890123456", "1234/../../evil", "１２３４"} {
		c := comicClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid ID sent request") })
		if _, err := c.GetComicReader(context.Background(), id); !errors.Is(err, sdk.ErrParamInvalid) {
			t.Errorf("%s: %v", id, err)
		}
	}
	t.Run("redirect", func(t *testing.T) {
		c := comicClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/comic/"+comicID {
				t.Error("followed redirect")
			}
			http.Redirect(w, r, "/redirected", 302)
		})
		if _, err := c.GetComicReader(context.Background(), comicID); err == nil {
			t.Fatal("accepted redirect")
		}
	})
	for name, body := range map[string]string{"oversize": strings.Repeat("x", (4<<20)+1), "missing loader": "<html></html>", "wrong loader type": `<script>_ROUTER_DATA = {"loaderData":[]};</script>`} {
		t.Run(name, func(t *testing.T) {
			if _, err := comicClient(t, htmlHandler(body)).GetComicReader(context.Background(), comicID); err == nil {
				t.Fatal("accepted bad body")
			}
		})
	}
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := comicClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("cancelled request sent") }).GetComicReader(ctx, comicID)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("wrong mime", func(t *testing.T) {
		c := comicClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, comicHTML(comicLoader()))
		})
		if _, err := c.GetComicReader(context.Background(), comicID); err == nil {
			t.Fatal("accepted non-html")
		}
	})
}
