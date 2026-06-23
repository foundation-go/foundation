package http

// Standard HTTP headers, that Foundation uses.
const (
	HeaderAcceptEncoding             = "Accept-Encoding"
	HeaderAccept                     = "Accept"
	HeaderAccessControlAllowHeaders  = "Access-Control-Allow-Headers"
	HeaderAccessControlAllowMethods  = "Access-Control-Allow-Methods"
	HeaderAccessControlAllowOrigin   = "Access-Control-Allow-Origin"
	HeaderAccessControlExposeHeaders = "Access-Control-Expose-Headers"
	HeaderAccessControlMaxAge        = "Access-Control-Max-Age"
	HeaderAuthorization              = "Authorization"
	HeaderContentLength              = "Content-Length"
	HeaderContentType                = "Content-Type"
	HeaderResponseType               = "ResponseType"
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

	// HeaderXWorkspaceID contains the workspace ID.
	HeaderXWorkspaceID = "X-Workspace-Id"
)

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
	HeaderXWorkspaceID,
}
