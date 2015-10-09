package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"golang.org/x/net/context"

	. "github.com/zach-klippenstein/memebot"
)

const (
	DefaultPort = 8080

	SlackTokenVar = "SLACK_TOKEN"

	DefaultKeywordPattern = `(\w+)$`
)

var ImageExtensions = []string{"jpg", "png", "gif"}

var (
	ImagesDir = flag.String("images", "",
		"path of `directory` containing images named like keyword1[,keyword2,...].")

	KeywordPattern = flag.String("keyword-pattern", DefaultKeywordPattern,
		"case-insensitive `regex` with capture groups used to extract keywords from messages.")

	ImageServerHostname = flag.String("serve-host", "",
		"`hostname` to use in image links.")

	ImageServerPort = flag.Int("serve-port", DefaultPort,
		"`port` to listen on for serving images.")

	ImageServerDisplayPort = flag.Int("serve-display-port", 0,
		"`port` use in image links. Maybe be different from -serve-port if your load balancer forwards 80 to 5000, e.g. Defaults to serve-port.")

	OnlyReplyToMentions = flag.Bool("require-mention", true,
		"if true, messages that don't mention bot will be ignored. If you set this, make sure to specify keyword-pattern!")

	ListKeywordsMode = flag.Bool("list-keywords", false,
		"lists the set of keywords without starting the bot")

	ListMemesMode = flag.Bool("list-memes", false,
		"lists all memes' URLs")

	ServeOnlyMode = flag.Bool("serve-only", false,
		"runs the image server without the bot for debugging.")
)

func init() {
	flag.Usage = func() {
		name := filepath.Base(os.Args[0])
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, name, "-images path [options...]")
		fmt.Fprintln(os.Stderr, name, "-list-keywords")
		fmt.Fprintln(os.Stderr, name, "-list-memes")
		fmt.Fprintln(os.Stderr)
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "The images directory is required, everything else is optional.")
	}
}

func main() {
	flag.Parse()

	if *ImagesDir == "" {
		flag.Usage()
		os.Exit(1)
	}

	if *ImageServerHostname == "" {
		host, err := os.Hostname()
		if err != nil {
			log.Fatal("error getting hostname:", err)
		}
		*ImageServerHostname = host
	}

	if *ImageServerDisplayPort == 0 {
		*ImageServerDisplayPort = *ImageServerPort
	}

	router := initRouter(*ImageServerHostname, *ImageServerDisplayPort)
	rootRoute := router.PathPrefix("/memes/")
	memepository := NewFileServingMemepository(FileServingMemepositoryConfig{
		Path:            *ImagesDir,
		ImageExtensions: MakeSet(ImageExtensions...),
		Router:          rootRoute.Subrouter(),
	})

	memes, err := memepository.Load()
	if err != nil {
		log.Fatal("error loading memes:", err)
	}

	if *ListKeywordsMode {
		for _, keyword := range memes.Keywords() {
			fmt.Printf("%s (%d)\n", keyword, len(memes.FindByKeyword(keyword)))
		}
		os.Exit(0)
	}

	if *ListMemesMode {
		for _, meme := range memes.Memes() {
			fmt.Printf("%s (%s)\n", meme.URL(), strings.Join(meme.Keywords(), ","))
		}
		os.Exit(0)
	}

	port := ":" + strconv.Itoa(*ImageServerPort)
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatal("image server error:", err)
	}
	rootUrl, _ := rootRoute.URL()
	log.Println("image server listening on port", port)
	log.Println("serving images on", rootUrl)

	defer func() {
		log.Println("exiting...")
	}()

	if *ServeOnlyMode {
		err = http.Serve(listener, router)
		if err != nil {
			log.Fatal("image server error:", err)
		}
	} else {
		go func() {
			err := http.Serve(listener, router)
			if err != nil {
				log.Fatal("image server error:", err)
			}
		}()

		startBot(memepository)
	}
}

func initRouter(hostname string, displayPort int) *mux.Router {
	routerAddr := fmt.Sprintf("%s:%d", hostname, displayPort)
	router := mux.NewRouter().Host(routerAddr).Subrouter()

	// Basic "are you alive?" check.
	router.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		log.Printf("health check from %s (X-Forwarded-For: %s)", req.RemoteAddr, req.Header["X-Forwarded-For"])
		fmt.Fprintln(w, "Everything looks good!")
		return
	})

	return router
}

func startBot(memepository Memepository) {
	slackToken := os.Getenv(SlackTokenVar)
	if slackToken == "" {
		log.Fatal("Slack token not found. Set ", SlackTokenVar)
	}

	parser, err := NewRegexpKeywordParser(*KeywordPattern)
	if err != nil {
		log.Fatalf("error compiling keyword pattern '%s': %s", *KeywordPattern, err)
	}

	if !*OnlyReplyToMentions {
		log.Println("WARNING: filtering by mentions is disabled. may be spammy.")
	}

	log.Println("connecting to slack...")
	bot, err := NewMemeBot(slackToken, MemeBotConfig{
		Parser:           MessageParser{KeywordParser: parser},
		Searcher:         &MemepositorySearcher{memepository},
		ParseAllMessages: !*OnlyReplyToMentions,
		Log:              log.New(os.Stderr, "", log.LstdFlags),
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Print("memebot ready as @", bot.Name(), " (^c to exit)")
	log.Println("matching keywords on", parser)

	bot.Run(context.Background())
}
