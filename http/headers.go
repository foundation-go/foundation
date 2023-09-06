package http

// Standard HTTP headers
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
	// HeaderXAuthenticated is the header that indicates if the request is authenticated
	HeaderXAuthenticated = "X-Authenticated"
	// HeaderXClientID is the header that contains the ID of the OAuth client
	HeaderXClientID = "X-Client-Id"
	// HeaderXCorrelationID is the header that contains the correlation ID
	HeaderXCorrelationID = "X-Correlation-Id"
	// HeaderXPage is the header that contains the current page
	HeaderXPage = "X-Page"
	// HeaderXPerPage is the header that contains the number of items per page
	HeaderXPerPage = "X-Per-Page"
	// HeaderXTotal is the header that contains the total number of items
	HeaderXTotal = "X-Total"
	// HeaderXTotalPages is the header that contains the total number of pages
	HeaderXTotalPages = "X-Total-Pages"
	// HeaderXUserID is the header that contains the user ID
	HeaderXUserID = "X-User-Id"
)

// All the Foundation headers
var FoundationHeaders = []string{
	HeaderXAuthenticated,
	HeaderXClientID,
	HeaderXCorrelationID,
	HeaderXPage,
	HeaderXPerPage,
	HeaderXTotal,
	HeaderXTotalPages,
	HeaderXUserID,
}
