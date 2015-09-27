
# memebot [![Build Status](https://travis-ci.org/zach-klippenstein/memebot.svg)](https://travis-ci.org/zach-klippenstein/memebot)

`memebot` is Slack bot that posts images matching keywords.

Images are currently served only out of a local directory, but S3 support could happen.

Slack integration code is based on https://github.com/rapidloop/mybot (see [slack.go](slack.go)).

## Installing

You'll need go 1.5.1 or later, and a properly-configured `GOPATH`.

    export GO15VENDOREXPERIMENT=1
    go get github.com/zach-klippenstein/memebot/cmd/memebot

## Running

First, you need to set your Slack API token:

    export SLACK_TOKEN=xxxx-xxxxxx…

Then start the bot with command-line config:

    memebot \
        -images /var/memes \
        -serve-port 8080 \
        -serve-host my-public-domain.com \
        -serve-display-port 80
        -keyword-pattern "show me (.+)"

Run `memebot -h` to see usage information.

You can also dump information about the meme repository:

    memebot -images /var/memes -list-keywords
    memebot -images /var/memes -list-memes

## Deploying to Heroku

Once you've created your Heroku app, set the `GO15VENDOREXPERIMENT` and other environment variables as described above. Note: `PORT` is set by Heroku, don't set it yourself. `DISPLAY_PORT` should be 80 (the public-facing port on Heroku's load balancer), and `HOSTNAME` should be whatever hostname your app is given (e.g. `my-memebot.heroku.com`).

To actually deploy the app, you will need to add the Heroku remote to your repo, create a Procfile, and push to master. I recommend creating a `deploy` branch and only adding your Procfile to that branch. You can also create a directory for images and add those. This allows you to keep your local master in sync with this repo (or whatever remote you're using), and merge your local master into your local deploy branch. 

Here's an example of how you'd set this all up (assuming your Heroku app name is my-memebot):

    cd $GOPATH/src/github.com/zach-klippenstein/memebot
    git checkout -b deploy
    cat >Procfile <<EOF
    web memebot -images $IMAGES_DIR -serve-display-port 80 -serve-host $HOSTNAME -keyword-pattern "$KEYWORD_PATTERN"
    EOF
    mkdir images
    # Add image files.
    git add --all && git commit -m "Initial deployment."
    heroku git:remote -a my-memebot
    
Then to actually deploy, just run:
    
    git push heroku deploy:master
