
# memebot 

`memebot` is Slack bot that posts images matching keywords.

Images are currently served only out of a local directory, but S3 support could happen.

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

…or using environment variables:

    export IMAGES_DIR=/var/memes
    export PORT=8080
    export HOSTNAME=my-public-domain.com
    export DISPLAY_PORT=80
    export KEYWORD_PATTERN="show me (.+)"
    memebot

Run `memebot -h` to see usage information.

You can also dump information about the meme repository:

    export IMAGES_DIR=/var/memes
    memebot -list-keywords
    memebot -list-memes

## Deploying to Heroku

Once you've created your Heroku app, set the `GO15VENDOREXPERIMENT` and other environment variables as described above. Note: `PORT` is set by Heroku, don't set it yourself. `DISPLAY_PORT` should be 80 (the public-facing port on Heroku's load balancer), and `HOSTNAME` should be whatever hostname your app is given (e.g. `my-memebot.heroku.com`).

To actually deploy the app, you will need to add the Heroku remote to your repo, create a Procfile, and push to master. I recommend creating a `deploy` branch and only adding your Procfile to that branch. You can also create a directory for images and add those. This allows you to keep your local master in sync with this repo (or whatever remote you're using), and merge your local master into your local deploy branch. 

Here's an example of how you'd set this all up (assuming your Heroku app name is my-memebot):

    cd $GOPATH/src/github.com/zach-klippenstein/memebot
    git checkout -b deploy
    echo "web: memebot" >Procfile
    mkdir images
    # Add image files.
    git add --all && git commit -m "Initial deployment."
    heroku git:remote -a my-memebot
    
Then to actually deploy, just run:
    
    git push heroku deploy:master
