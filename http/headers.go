package http

// Standard HTTP headers, that Foundation uses.
const (
	HeaderAcceptEncoding                = "Accept-Encoding"
	HeaderAccept                        = "Accept"
	HeaderAccessControlAllowCredentials = "Access-Control-Allow-Credentials"
	HeaderAccessControlAllowHeaders     = "Access-Control-Allow-Headers"
	HeaderAccessControlAllowMethods     = "Access-Control-Allow-Methods"
	HeaderAccessControlAllowOrigin      = "Access-Control-Allow-Origin"
	HeaderAccessControlExposeHeaders    = "Access-Control-Expose-Headers"
	HeaderAccessControlMaxAge           = "Access-Control-Max-Age"
	HeaderAuthorization                 = "Authorization"
	HeaderContentLength                 = "Content-Length"
	HeaderContentType                   = "Content-Type"
	HeaderResponseType                  = "ResponseType"
)

// Foundation HTTP headers
const (
	// HeaderXAuthenticated indicates if the request is authenticated.
	HeaderXAuthenticated = "X-Authenticated"

	// HeaderXClientID contains the ID of the OAuth client.
	HeaderXClientID = "X-Client-Id"

	// HeaderXCorrelationID contains the correlation ID.
	HeaderXCorrelationID = "X-Correlation-Id"

	// HeaderXPage contains the current page.
	HeaderXPage = "X-Page"

	// HeaderXPerPage contains the number of items per page.
	HeaderXPerPage = "X-Per-Page"

	// HeaderXScope contains the OAuth scope.
	HeaderXScope = "X-Scope"

	// HeaderXMetadata contains the additional metadata for the authenticated user.
	HeaderXMetadata = "X-Metadata"

	// HeaderXTotal contains the total number of items.
	HeaderXTotal = "X-Total"

	// HeaderXTotalPages contains the total number of pages
	HeaderXTotalPages = "X-Total-Pages"

	// HeaderXUserID contains the user ID.
	HeaderXUserID = "X-User-Id"

	// HeaderXRequestID contains the request ID.
	HeaderXRequestID = "X-Request-Id"
)

// AuthenticationHeaders is the subset of Foundation headers that carries the
// caller's identity.
//
// These headers are produced by the gateway from the result of authenticating
// the request, and are trusted by every downstream service. They MUST NOT be
// accepted from clients: a request that arrives with any of them set is
// attempting to impersonate somebody. The gateway strips them from every
// incoming request before the middleware chain runs.
var AuthenticationHeaders = []string{
	HeaderXAuthenticated,
	HeaderXClientID,
	HeaderXMetadata,
	HeaderXScope,
	HeaderXUserID,
}

// FoundationHeaders is a list of all Foundation HTTP headers.
//
// Use this list when referencing all Foundation HTTP headers in your code.
var FoundationHeaders = []string{
	HeaderXAuthenticated,
	HeaderXClientID,
	HeaderXCorrelationID,
	HeaderXMetadata,
	HeaderXPage,
	HeaderXPerPage,
	HeaderXScope,
	HeaderXTotal,
	HeaderXTotalPages,
	HeaderXUserID,
	HeaderXRequestID,
}
