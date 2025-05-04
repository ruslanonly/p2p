module traffic

go 1.23.1

require github.com/james-barrow/golang-ipc v1.2.4

require pkg v0.0.0

replace pkg => ../pkg

require (
	github.com/Microsoft/go-winio v0.6.1 // indirect
	golang.org/x/mod v0.10.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/tools v0.9.1 // indirect
)
