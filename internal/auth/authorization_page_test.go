package auth

import "testing"

func TestPageIsNotFoundRecognisesRealPages(t *testing.T) {
	// A framework not-found page names itself in the document title.
	notFound := `<!DOCTYPE html><html><head><title>404: This page could not be found.</title></head>
		<body><h1>404</h1><p>This page could not be found.</p></body></html>`
	if !pageIsNotFound(notFound) {
		t.Fatal("a not-found page was treated as available")
	}

	// The authorization page answers 200 and carries the framework's
	// not-found text inside its own payload. Rejecting it there is what left
	// engineers unable to connect the Career Agent.
	authorizationPage := `<!DOCTYPE html><html><head>
		<title>Proofboard | Verified Career Infrastructure for Engineers</title></head>
		<body><div id="cli-auth">Authorize this device</div>
		<script>self.__next_f.push([1,"This page could not be found"])</script>
		</body></html>`
	if pageIsNotFound(authorizationPage) {
		t.Fatal("the authorization page was rejected because of framework payload text")
	}
}

func TestPageIsNotFoundWithoutTitle(t *testing.T) {
	if !pageIsNotFound(`<html><body>DEPLOYMENT_NOT_FOUND</body></html>`) {
		t.Fatal("a missing deployment was treated as available")
	}
	if pageIsNotFound(`<html><body>Authorize this device</body></html>`) {
		t.Fatal("a page without a title was rejected")
	}
}
