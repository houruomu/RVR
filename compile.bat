env GOOS=linux GOARCH=amd64 go build
env GOOS=windows GOARCH=amd64 go build
git add *
git commit -m "Version 0.4.2 greatly relax the testing environment to have a easier time."
git push

exit