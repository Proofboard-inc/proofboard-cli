package commands

import "testing"

// VS Code records the open folder as a file URI. On Windows that URI carries
// the drive letter inside the path — "file:///c%3A/Users/ada/project" — and the
// leading slash in "/c:/Users/ada/project" is part of the URI, not the path.
//
// Handing that straight to filepath.Abs produced something like
// C:\c:\Users\ada\project, which matches no repository, so editor-based
// workspace discovery could never find anything on Windows. The failure is
// silent: discovery simply returns nothing, which is indistinguishable from
// "no editor is open" and looks exactly like detection having stopped working.
//
// goos is a parameter so both conventions are covered from any host; a test
// that could only assert the Windows form while running on Windows would have
// left this exactly as undiscovered as it was.
func TestWorkspacePathFromFileURI(t *testing.T) {
	for _, tc := range []struct {
		name string
		uri  string
		goos string
		want string
	}{
		{
			name: "windows drive letter, percent-encoded as VS Code writes it",
			uri:  "file:///c%3A/Users/ada/project",
			goos: "windows",
			want: `c:\Users\ada\project`,
		},
		{
			name: "windows drive letter, unencoded",
			uri:  "file:///C:/Users/ada/project",
			goos: "windows",
			want: `C:\Users\ada\project`,
		},
		{
			name: "windows UNC-style path keeps its root",
			uri:  "file:///C:/",
			goos: "windows",
			want: `C:\`,
		},
		{
			name: "posix path is unchanged",
			uri:  "file:///home/ada/project",
			goos: "linux",
			want: "/home/ada/project",
		},
		{
			name: "posix path with a space",
			uri:  "file:///home/ada/my%20project",
			goos: "darwin",
			want: "/home/ada/my project",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := workspacePathFromFileURI(tc.uri, tc.goos)
			if got != tc.want {
				t.Fatalf("workspacePathFromFileURI(%q, %q) = %q, want %q",
					tc.uri, tc.goos, got, tc.want)
			}
		})
	}
}
