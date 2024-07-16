module contact-form

go 1.22

toolchain go1.22.5

require (
	github.com/alexedwards/scs/boltstore v0.0.0-20240316134038-7e11d57e8885
	github.com/alexedwards/scs/v2 v2.8.0
	github.com/dchest/captcha v1.0.0
	github.com/gorilla/csrf v1.7.2
	github.com/nelkinda/health-go v0.0.1
	go.etcd.io/bbolt v1.3.10
)

require github.com/csmith/envflag v1.0.0

require (
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/nelkinda/http-go v0.0.1 // indirect
	golang.org/x/sys v0.22.0 // indirect
)
