env GOOS=linux GOARCH=amd64 go build
env GOOS=windows GOARCH=amd64 go build
git add *
git commit -m "change version number"
git push

exit