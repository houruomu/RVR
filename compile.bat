env GOOS=linux GOARCH=amd64 go build
env GOOS=windows GOARCH=amd64 go build
git add *
git commit -m "Version 0.3.10. Last try of the day: reduce test size for a reliable test"
git push

exit