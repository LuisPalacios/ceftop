package icons

import "testing"

func TestScoreTiers(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		// 100: identical after lowercase / .exe strip / path strip
		{"chrome", "chrome", 100},
		{"Chrome.exe", "chrome", 100},
		{`C:\Program Files\Google\chrome.exe`, "CHROME", 100},
		{"Docker Desktop.exe", "docker desktop", 100},

		// 95: separators are irrelevant
		{"Docker Desktop", "docker-desktop", 95},
		{"docker desktop", "dockerdesktop", 95},
		{"Docker_Desktop", "Docker.Desktop", 95},
		{"sumwall.browser", "Sumwall Browser", 95},

		// 85: platform / glue noise is irrelevant
		{"docker desktop for mac", "docker desktop", 85},
		{"Win Docker desKtop", "Docker-desktop", 85},
		{"Docker Desktop for Windows x64", "docker desktop", 85},
		{"vivaldi-bin", "vivaldi", 85},
		{"Brave Browser.app", "brave browser", 85},

		// 50..80: token containment, scaled by coverage
		{"google chrome", "chrome", 65},
		{"microsoft edge", "edge", 65},
		{"Docker Desktop", "docker", 65},
		{"desktop docker", "docker desktop", 80}, // same set, different order
		{"visual studio code insiders", "visual studio code", 72},

		// 40: glued prefix
		{"msedge", "msedgewebview2", 40},
		{"chromiumbrowser", "chromium", 40}, // single glued token, so only the prefix rule can fire

		// 0: unrelated
		{"chrome", "edge", 0},
		{"code", "cursor", 0},
		{"edge", "msedgewebview2", 0}, // "edge" is not a prefix of "msedgewebview2"
		{"slack", "", 0},
		{"", "", 0},
		{"go", "google", 0}, // prefix shorter than 4 chars never matches
	}
	for _, c := range cases {
		got := Score(c.a, c.b)
		if got != c.want {
			t.Errorf("Score(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
		if back := Score(c.b, c.a); back != got {
			t.Errorf("Score is not symmetric: (%q,%q)=%d but (%q,%q)=%d", c.a, c.b, got, c.b, c.a, back)
		}
	}
}

func TestScoreChromiumBrowserUsesContainment(t *testing.T) {
	// Sanity check on the one case above that is easy to misread: "chromium
	// browser" and "chromium" share every meaningful token of the shorter
	// side, so containment (not the prefix rule) fires.
	if got := Score("chromium-browser", "chromium"); got != 65 {
		t.Fatalf("Score(chromium-browser, chromium) = %d, want 65", got)
	}
}

func TestTokensAndCanonical(t *testing.T) {
	got := Tokens("Docker-Desktop for Mac (x64).exe")
	want := []string{"docker", "desktop", "for", "mac", "x64", "exe"}
	if len(got) != len(want) {
		t.Fatalf("Tokens = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Tokens = %v, want %v", got, want)
		}
	}
	if c := Canonical("Docker-Desktop for Mac (x64).exe"); c != "dockerdesktop" {
		t.Fatalf("Canonical = %q, want dockerdesktop", c)
	}
	// A name made only of noise keeps its identity instead of vanishing.
	if c := Canonical("app"); c != "app" {
		t.Fatalf("Canonical(app) = %q, want app", c)
	}
}

func TestBestMatchPrefersStrongestThenMostSpecific(t *testing.T) {
	keys := []string{"chrome", "docker", "docker-desktop", "edge", "msedgewebview2", "code"}

	cases := []struct {
		name    string
		wantKey string
		wantOK  bool
	}{
		{"Docker Desktop.exe", "docker-desktop", true}, // 95 beats 65
		{"docker", "docker", true},                     // exact beats containment
		{"Google Chrome", "chrome", true},
		{"msedgewebview2.exe", "msedgewebview2", true},
		{"Microsoft Edge", "edge", true},
		{"Code", "code", true},
		{"cursor", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		key, _, ok := BestMatch(c.name, keys)
		if ok != c.wantOK || key != c.wantKey {
			t.Errorf("BestMatch(%q) = (%q, %v), want (%q, %v)", c.name, key, ok, c.wantKey, c.wantOK)
		}
	}
}

func TestBestMatchIsDeterministicOnTies(t *testing.T) {
	// Both keys score 95 against the name; the longer key wins, and when
	// lengths tie the lexically smaller one does — regardless of input order.
	name := "docker desktop"
	a := []string{"docker-desktop", "docker_desktop"}
	b := []string{"docker_desktop", "docker-desktop"}
	ka, _, _ := BestMatch(name, a)
	kb, _, _ := BestMatch(name, b)
	if ka != kb || ka != "docker-desktop" {
		t.Fatalf("tie-break unstable: %q vs %q (want docker-desktop)", ka, kb)
	}
}
