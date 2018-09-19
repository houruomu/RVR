env GOOS=linux GOARCH=amd64 go build
env GOOS=windows GOARCH=amd64 go build
git add *
git commit -m "additional monitoring at the beginning to prevent dissynchronization"
git push

exit