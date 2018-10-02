env GOOS=linux GOARCH=amd64 go build
env GOOS=windows GOARCH=amd64 go build
git add *
git commit -m "Version 0.4.8 fix bugs in malicious counting"
git push

exit

spawn
