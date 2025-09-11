# Changelog

## 2.2.0 - 2025-09-11

### Major changes

- Added support for spam checking via OOPSpam

## 2.1.1 - 2025-08-13

### Bug fixes

- Fixed errors when attempting to solve captchas. This was a result of switching
  the CSRF implementation in v2.1.0; the old implementation implicitly parsed
  forms, the new one didn't, and the captcha behaviour relied on this.

### Other changes

- Dependency updates
- Improved a couple of log lines

## 2.1.0 - 2025-05-18

### Major changes

- CSRF protection now relies on the `Sec-Fetch-Site` header sent by all modern
  browsers.
  - CSRF cookies are no longer used
  - `{{.csrfField}}` is no longer required in templates
    - it will evaluate to an empty string for compatibility with existing templates
  - Connecting to a development version on `localhost` now works without code modifications
  - Removes dependency on gorilla/csrf

### Other changes

- Added more extensive logging. This can be customised using the `log.level` and
  `log.format` flags (or equivalent env vars)
  - At `DEBUG` level (disabled by default), e-mail addresses will appear in logs
- The HTTP server is now shutdown correctly, allowing clients to finish requests
  that are in-flight
- Minor dependency updates

## 2.0.0 - 2024-07-16

### Breaking changes

- Templates are now stored and read from a `templates` subdirectory
- Static files in the `static` subdirectory are now served at `/static/`
- The default CSS used in the templates is no longer inlined and duplicated and
  is now in `static/style.css` (this makes it much easier to override just the
  page style if you want to customise it!)

### Other changes

- Minor dependency version updates
- Replaced gorilla/mux with Go's built in http.ServeMux

## 1.0.1 - 2023-02-13

### Other changes

- Minor dependency updates

## 1.0.0 - 2021-09-01

_Initial release._