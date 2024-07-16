# Changelog

## vNext

### Breaking changes

- Templates are now stored and read from a `templates` subdirectory
- Static files in the `static` subdirectory are now served at `/static/`
- The default CSS used in the templates is no longer inlined and duplicated and
  is now in `static/style.css` (this makes it much easier to override just the
  page style if you want to customise it!)

### Other changes

- Minor dependency version updates
- Replaced gorilla/mux with Go's built in http.ServeMux

## v1.0.1

- Minor dependency updates