# Changelog

## 1.5.3

- Built on go-scrapfly v1.8.0, up from v1.7.0, which go-scrapfly retracted.
  Every tool sends the same requests and returns the same results as in
  1.5.2, and the tool names, input and output schemas, annotations and server
  capabilities are unchanged.
- The alert tools' descriptions say which project an API key reaches:
  `alert_list` and `alert_count_active` read the key's own project,
  `alert_preview` previews it and `alert_create` binds the alert to it. A
  `project_uuid` naming another project lists or counts nothing, and
  `alert_create` refuses it with `ERR::ALERT::PROJECT_NOT_FOUND`. Only the
  descriptions changed.
- The embedded Web Scraping API reference says that `url` and the `headers`
  values are URL-encoded once, as part of the query string.

## Earlier releases

Not tracked in this file: see the repository's tags.
