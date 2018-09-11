env GOOS=linux GOARCH=amd64 go build
env GOOS=windows GOARCH=amd64 go build
git add *
git commit -m "add watchdog for connection states and fail gracefully"
git push

exit