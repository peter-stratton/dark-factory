// etag-cache.js — Teaches HTMX to use ETag/If-None-Match for polling requests.
// Stores the ETag from each HTMX response and sends it back on the next
// request to the same URL. When the server returns 304 Not Modified, the
// swap is skipped entirely, preventing unnecessary DOM updates.
(function() {
  var etags = {};

  // Tell HTMX not to swap on 304 responses.
  htmx.config.responseHandling = [
    {code: "204", swap: false},
    {code: "304", swap: false},
    {code: "[23]..", swap: true},
    {code: "[45]..", swap: false, error: true}
  ];

  // Before each HTMX request, attach the stored ETag as If-None-Match.
  document.addEventListener("htmx:configRequest", function(evt) {
    var path = evt.detail.path;
    if (etags[path]) {
      evt.detail.headers["If-None-Match"] = etags[path];
    }
  });

  // After each HTMX response, store the ETag for next time.
  document.addEventListener("htmx:afterRequest", function(evt) {
    var xhr = evt.detail.xhr;
    if (!xhr) return;
    var path = evt.detail.pathInfo && evt.detail.pathInfo.requestPath
            || evt.detail.requestConfig && evt.detail.requestConfig.path
            || "";
    var etag = xhr.getResponseHeader("ETag");
    if (etag && path) {
      etags[path] = etag;
    }
  });
})();
