package netease

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/bjarneo/cliamp/provider"
)

func TestPlaylistsIncludesAccountListsAndCharts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/playlist" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("uid"); got != "42" {
			t.Fatalf("uid = %q, want 42", got)
		}
		w.Write([]byte(`{"code":200,"playlist":[
			{"id":10,"name":"Daily Picks","userId":42,"trackCount":12,"specialType":5},
			{"id":11,"name":"Road Trip","userId":42,"trackCount":8,"specialType":0},
			{"id":12,"name":"Saved Mix","userId":99,"trackCount":20,"specialType":0}
		]}`))
	}))
	defer srv.Close()

	p := newWithBase(Config{Enabled: true, UserID: "42"}, srv.URL)
	lists, err := p.Playlists()
	if err != nil {
		t.Fatalf("Playlists() error = %v", err)
	}
	if len(lists) != 9 {
		t.Fatalf("got %d playlists, want 9", len(lists))
	}
	if lists[0].ID != "recommend:daily" || lists[0].Name != "Daily Recommendation" || lists[0].Section != "Discover" {
		t.Fatalf("daily recommendation playlist = %+v", lists[0])
	}
	if lists[1].ID != "radar:personal" || lists[1].Name != "Personal Radar" || lists[1].Section != "Discover" {
		t.Fatalf("personal radar playlist = %+v", lists[1])
	}
	if lists[2].ID != "user:10" || lists[2].Name != "Liked Songs" || lists[2].Section != "My Playlists" {
		t.Fatalf("liked playlist = %+v", lists[2])
	}
	if lists[4].Section != "Saved Playlists" {
		t.Fatalf("saved playlist section = %q", lists[4].Section)
	}
	if lists[5].ID != "chart:3778678" || lists[5].Section != "Charts" {
		t.Fatalf("first chart = %+v", lists[5])
	}
}

func TestTracksMapsSongs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/playlist/detail" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("id"); got != "10" {
			t.Fatalf("id = %q, want 10", got)
		}
		w.Write([]byte(`{"code":200,"result":{"tracks":[
			{"id":100,"name":"First Track","duration":123456,"no":3,
			 "artists":[{"name":"Artist One"},{"name":"Artist Two"}],
			 "album":{"name":"Album One"}}
		]}}`))
	}))
	defer srv.Close()

	p := newWithBase(Config{Enabled: true}, srv.URL)
	tracks, err := p.Tracks("user:10")
	if err != nil {
		t.Fatalf("Tracks() error = %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("got %d tracks, want 1", len(tracks))
	}
	tr := tracks[0]
	if tr.Path != "https://music.163.com/#/song?id=100" {
		t.Fatalf("Path = %q", tr.Path)
	}
	if tr.Artist != "Artist One, Artist Two" || tr.Album != "Album One" {
		t.Fatalf("metadata = artist %q album %q", tr.Artist, tr.Album)
	}
	if tr.DurationSecs != 124 {
		t.Fatalf("DurationSecs = %d, want 124", tr.DurationSecs)
	}
	if tr.Meta(provider.MetaNetEaseID) != "100" {
		t.Fatalf("MetaNetEaseID = %q", tr.Meta(provider.MetaNetEaseID))
	}
}

func TestSearchTracks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search/get/web" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("s"); got != "query" {
			t.Fatalf("search query = %q, want query", got)
		}
		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Fatalf("limit = %q, want 5", got)
		}
		w.Write([]byte(`{"code":200,"result":{"songs":[
			{"id":200,"name":"Search Hit","duration":1000,
			 "artists":[{"name":"Artist"}],"album":{"name":"Album"}}
		]}}`))
	}))
	defer srv.Close()

	p := newWithBase(Config{Enabled: true}, srv.URL)
	tracks, err := p.SearchTracks(context.Background(), " query ", 5)
	if err != nil {
		t.Fatalf("SearchTracks() error = %v", err)
	}
	if len(tracks) != 1 || tracks[0].Title != "Search Hit" {
		t.Fatalf("tracks = %+v", tracks)
	}
}

func TestCookieHeaderFromNetscapeFileFiltersNetEaseCookies(t *testing.T) {
	path := t.TempDir() + "/cookies.txt"
	data := strings.Join([]string{
		"# Netscape HTTP Cookie File",
		".music.163.com\tTRUE\t/\tTRUE\t0\tMUSIC_U\tabc",
		"#HttpOnly_.163.com\tTRUE\t/\tTRUE\t0\t__csrf\tdef",
		".example.com\tTRUE\t/\tTRUE\t0\tOTHER\tignored",
		"",
	}, "\n")
	if err := osWriteFile(path, data); err != nil {
		t.Fatal(err)
	}
	header, err := cookieHeaderFromNetscapeFile(path)
	if err != nil {
		t.Fatalf("cookieHeaderFromNetscapeFile() error = %v", err)
	}
	if header != "MUSIC_U=abc; __csrf=def" {
		t.Fatalf("header = %q", header)
	}
}

func TestExtractBrowserCookieHeaderMissingYTDLPShowsInstallHint(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := extractBrowserCookieHeader(context.Background(), "chrome")
	if err == nil {
		t.Fatal("extractBrowserCookieHeader() error = nil, want missing yt-dlp error")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "yt-dlp not found. Install with: ") {
		t.Fatalf("error = %q", msg)
	}
	if strings.TrimPrefix(msg, "yt-dlp not found. Install with: ") == "" {
		t.Fatalf("missing install hint in error = %q", msg)
	}
}

func TestLiveCheckLoginWithBrowser(t *testing.T) {
	browser := os.Getenv("CLIAMP_NETEASE_LIVE_BROWSER")
	if browser == "" {
		t.Skip("set CLIAMP_NETEASE_LIVE_BROWSER to run live browser-cookie check")
	}
	acc, err := CheckLogin(context.Background(), browser)
	if err != nil {
		t.Fatalf("CheckLogin() error = %v", err)
	}
	if acc.UserID == "" {
		t.Fatal("CheckLogin() returned empty user id")
	}
}

func osWriteFile(path, data string) error {
	return os.WriteFile(path, []byte(data), 0o644)
}

func TestTracksDailyRecommendation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/discovery/recommend/songs" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"code":200,"data":{"dailySongs":[
			{"id":1001,"name":"Daily Song One","dt":200000,"no":1,
			 "ar":[{"name":"Artist A"},{"name":"Artist B"}],
			 "al":{"name":"Album Alpha"}}
		]}}`))
	}))
	defer srv.Close()

	p := newWithBase(Config{Enabled: true}, srv.URL)
	tracks, err := p.Tracks("recommend:daily")
	if err != nil {
		t.Fatalf("Tracks(recommend:daily) error = %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("got %d tracks, want 1", len(tracks))
	}
	tr := tracks[0]
	if tr.Path != "https://music.163.com/#/song?id=1001" {
		t.Fatalf("Path = %q", tr.Path)
	}
	if tr.Title != "Daily Song One" || tr.Artist != "Artist A, Artist B" || tr.Album != "Album Alpha" {
		t.Fatalf("metadata mismatch: title %q artist %q album %q", tr.Title, tr.Artist, tr.Album)
	}
	if tr.DurationSecs != 200 {
		t.Fatalf("DurationSecs = %d, want 200", tr.DurationSecs)
	}
	if tr.Meta(provider.MetaNetEaseID) != "1001" {
		t.Fatalf("MetaNetEaseID = %q, want 1001", tr.Meta(provider.MetaNetEaseID))
	}
}

func TestTracksDailyRecommendationFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/discovery/recommend/songs" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"code":200,"recommend":[
			{"id":1002,"name":"Fallback Song","duration":180000,"no":2,
			 "artists":[{"name":"Artist C"}],
			 "album":{"name":"Album Beta"}}
		]}}`))
	}))
	defer srv.Close()

	p := newWithBase(Config{Enabled: true}, srv.URL)
	tracks, err := p.Tracks("recommend:daily")
	if err != nil {
		t.Fatalf("Tracks(recommend:daily) error = %v", err)
	}
	if len(tracks) != 1 || tracks[0].Title != "Fallback Song" {
		t.Fatalf("unexpected tracks = %+v", tracks)
	}
}

func TestTracksPersonalRadar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/playlist/detail" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("id"); got != "3136952023" {
			t.Fatalf("id = %q, want 3136952023", got)
		}
		w.Write([]byte(`{"code":200,"result":{"tracks":[
			{"id":3001,"name":"Radar Track","duration":180000,"no":1,
			 "artists":[{"name":"Radar Artist"}],
			 "album":{"name":"Radar Album"}}
		]}}`))
	}))
	defer srv.Close()

	p := newWithBase(Config{Enabled: true}, srv.URL)
	tracks, err := p.Tracks("radar:personal")
	if err != nil {
		t.Fatalf("Tracks(radar:personal) error = %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("got %d tracks, want 1", len(tracks))
	}
	if tracks[0].Title != "Radar Track" || tracks[0].Artist != "Radar Artist" {
		t.Fatalf("unexpected tracks = %+v", tracks)
	}
}

func TestCanRefreshPlaylist(t *testing.T) {
	p := New(Config{Enabled: true})
	if !p.CanRefreshPlaylist("recommend:daily") {
		t.Error("CanRefreshPlaylist(recommend:daily) = false, want true")
	}
	if !p.CanRefreshPlaylist("radar:personal") {
		t.Error("CanRefreshPlaylist(radar:personal) = false, want true")
	}
	if p.CanRefreshPlaylist("radio:fm") {
		t.Error("CanRefreshPlaylist(radio:fm) = true, want false")
	}
	if p.CanRefreshPlaylist("user:123") {
		t.Error("CanRefreshPlaylist(user:123) = true, want false")
	}
	if p.CanRefreshPlaylist("chart:3778678") {
		t.Error("CanRefreshPlaylist(chart:3778678) = true, want false")
	}
}
