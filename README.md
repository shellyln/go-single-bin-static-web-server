# go-single-bin-static-web-server

This is a template for a single-binary web server written in Go.  
All content is embedded in the binary of the executable file.
Simply deploying one executable file will make it work as a web server.

You can fork this project, store your content in the folder for static content,
commit, build, and deploy it.


## Usage

```bash
# Clone this repo
git clone https://github.com/shellyln/go-single-bin-static-web-server.git
cd go-single-bin-static-web-server

# Create a fork on GitHub on your account and change remote
gh repo fork --remote=true --fork-name my-server

# Fix module name
vi go.mod
cat go.mod
> module github.com/your-name/my-server
> go 1.18

# Setup your contents
vi static/index.html

# Commit changes
git commit -m "Edit something"
git push

# and build executable
make

# Run (port 8080)
./myhttpd

# Run (port 3000)
env PORT=3000 ./myhttpd

# If you want to use with docker, build a docker image
make docker

# and run
docker compose up
```


### Init script for OpenWrt

```bash
/etc/init.d/myhttpd enable
/etc/init.d/myhttpd start
/etc/init.d/myhttpd stop
/etc/init.d/myhttpd disable
```


## Features

* Serve static contents
    * Place contents in `static/`
    * Edit `mimetypes.json` to add custom content types
* Reverse proxy
    * Edit `revproxies.json`
      ```json
      [{
          "scheme": "http",
          "host": "127.0.0.1:3001",
          "path": "/api/v1/",
          "reqHeaders": {
              "X-Proxy-Req-Header": "hello"
          },
          "resHeaders": {
              "X-Proxy-Res-Header": "world"
          }
      }, {
          "scheme": "http",
          "host": "127.0.0.1:3002",
          "path": "/api/v2/",
          "reqHeaders": {
              "X-Proxy-Req-Header": "hello"
          },
          "resHeaders": {
              "X-Proxy-Res-Header": "world"
          }
      }]
      ```

## License

MIT  
Copyright (c) 2023 Shellyl_N and Authors.
