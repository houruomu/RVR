env GOOS=linux GOARCH=amd64 go build
env GOOS=windows GOARCH=amd64 go build
git add *
git commit -m "Version 0.3.19 improve the efficiency of sample, try to solve bugs in autotest monitoring"
git push

exit