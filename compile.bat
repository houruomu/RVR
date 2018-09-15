env GOOS=linux GOARCH=amd64 go build
env GOOS=windows GOARCH=amd64 go build
git add *
git commit -m "improve RPC call efficiency by setting timeout"
git push

exit