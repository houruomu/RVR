env GOOS=linux GOARCH=amd64 go build
env GOOS=windows GOARCH=amd64 go build
git add *
git commit -m "Version 0.4.4 stop nodes from dropping each other"
git push

exit

spawn
