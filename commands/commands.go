package commands

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nlopes/slack"
	"github.com/otium/ytdl"
)

var joinAPIKey = os.Getenv("joinAPIKey")
var raspberryPIIP = os.Getenv("raspberryPIIP")
var rtm *slack.RTM
var piSDCardPath = "/home/pi/torrents/"
var piUSBMountPath = "/mnt/usb_1/DLNA/torrents/"
var routerIP = "192.168.1.1"
var tranc = "tranc"

// SlackReportChannel default reporting channel for bot crons
var SlackReportChannel = os.Getenv("slackReportChannel") // C33QYV3PW is #remote_network_report

// SetRTM sets singleton
func SetRTM(rtmPassed *slack.RTM) {
	rtm = rtmPassed
}

// CheckCommand is now commented
func CheckCommand(api *slack.Client, slackMessage slack.Msg, command string) {
	args := strings.Fields(command)
	callingUserProfile, _ := api.GetUserInfo(slackMessage.User)
	params := slack.PostMessageParameters{AsUser: true}

	if args[0] == "yt" {
		if len(args) > 1 {
			// strip '<>' off url
			downloadURL := strings.Trim(args[1], "<>")
			uri, err := url.ParseRequestURI(downloadURL)
			if err != nil {
				api.PostMessage(slackMessage.Channel, "Invalid URL for downloading! ("+err.Error()+")", params)
			} else {
				result := sendPayloadToJoinAPI(uri.String())
				api.PostMessage(slackMessage.Channel, result, params)
			}
		} else {
			api.PostMessage(slackMessage.Channel, "Please provide YouTube video URL!", params)
		}
	} else if args[0] == "bb" {
		// TODO pass yesterday's date
		response := ShowBaseBallGames()
		result := "Ball games from " + response.ReadableDate + ":\n"

		for _, gameMetaData := range response.Games {
			watchURL := "<" + gameMetaData[10] + "|" + gameMetaData[0] + " @ " + gameMetaData[4] + ">    "
			downloadURL := "<https://ackerson.de/bb_download?gameTitle=" + gameMetaData[2] + "-" + gameMetaData[6] + "__" + response.ReadableDate + "&gameURL=" + gameMetaData[10] + " | :smartphone:>"

			result += watchURL + downloadURL + "\n"
		}

		api.PostMessage(slackMessage.Channel, result, params)
	} else if args[0] == "do" {
		response := ListDODroplets(true)
		api.PostMessage(slackMessage.Channel, response, params)
	} else if args[0] == "dd" {

		if len(args) > 1 {
			number, err := strconv.Atoi(args[1])
			if err != nil {
				api.PostMessage(slackMessage.Channel, "Invalid integer value for ID!", params)
			} else {
				result := DeleteDODroplet(number)
				api.PostMessage(slackMessage.Channel, result, params)
			}
		} else {
			api.PostMessage(slackMessage.Channel, "Please provide Droplet ID from `do` cmd!", params)
		}
	} else if args[0] == "fsck" {
		if runningFritzboxTunnel() {
			response := ""

			if len(args) > 1 {
				path := strings.Join(args[1:], " ")
				response += CheckPiDiskSpace(path)
			} else {
				response += CheckPiDiskSpace("")
			}

			rtm.SendMessage(rtm.NewOutgoingMessage(response, slackMessage.Channel))
		}
	} else if args[0] == "mv" || args[0] == "rm" {
		response := ""
		if len(args) > 1 {
			if runningFritzboxTunnel() {
				path := strings.Join(args[1:], " ")
				if args[0] == "rm" {
					response = DeleteTorrentFile(path)
				} else {
					MoveTorrentFile(path)
				}

				rtm.SendMessage(rtm.NewOutgoingMessage(response, slackMessage.Channel))
			}
		} else {
			rtm.SendMessage(rtm.NewOutgoingMessage("Please provide a filename", slackMessage.Channel))
		}
	} else if args[0] == "torq" {
		var response string
		cat := 0
		if len(args) > 1 {
			if args[1] == "nfl" {
				cat = 200
			} else if args[1] == "ubuntu" {
				cat = 300
			}

			searchString := strings.Join(args, " ")
			searchString = strings.TrimLeft(searchString, "torq")
			fmt.Println("searching for: " + searchString)
			_, response = SearchFor(searchString, Category(cat))
		} else {
			_, response = SearchFor("", Category(cat))
		}
		api.PostMessage(slackMessage.Channel, response, params)
	} else if args[0] == "ovpn" {
		response := RaspberryPIPrivateTunnelChecks(true)
		rtm.SendMessage(rtm.NewOutgoingMessage(response, slackMessage.Channel))
	} else if args[0] == "sw" {
		response := ":partly_sunny_rain: <https://www.wunderground.com/cgi-bin/findweather/getForecast?query=" +
			"48.3,11.35#forecast-graph|10-day forecast Schwabhausen>"
		api.PostMessage(slackMessage.Channel, response, params)
	} else if args[0] == "vpnc" {
		result := vpnTunnelCmds("/usr/sbin/vpnc-connect", "fritzbox")
		rtm.SendMessage(rtm.NewOutgoingMessage(result, slackMessage.Channel))
	} else if args[0] == "vpnd" {
		result := vpnTunnelCmds("/usr/sbin/vpnc-disconnect")
		rtm.SendMessage(rtm.NewOutgoingMessage(result, slackMessage.Channel))
	} else if args[0] == "vpns" {
		result := vpnTunnelCmds("status")
		rtm.SendMessage(rtm.NewOutgoingMessage(result, slackMessage.Channel))
	} else if args[0] == "trans" || args[0] == "trand" || args[0] == tranc {
		if runningFritzboxTunnel() {
			response := torrentCommand(args)
			rtm.SendMessage(rtm.NewOutgoingMessage(response, slackMessage.Channel))
		}
	} else if args[0] == "mvv" {
		response := "<https://img.srv2.de/customer/sbahnMuenchen/newsticker/newsticker.html|Aktuelles>"

		if len(args) > 1 {
			if args[1] == "m" {
				// show next train to MUC
				response = "<" + mvvRoute("Schwabhausen", "München, Hauptbahnhof") + "|going to MUC>"
			} else if args[1] == "s" {
				// show next train to SCHWAB
				response = "<" + mvvRoute("München, Hauptbahnhof", "Schwabhausen") + "|going home>"
			}
		}

		api.PostMessage(slackMessage.Channel, response, params)
	} else if args[0] == "help" {
		response := ":sun_behind_rain_cloud: `sw`: Schwabhausen weather\n" +
			":metro: `mvv (s|m)`: no args->show status, `s`->come home, `m`->goto MUC\n" +
			":do_droplet: `do|dd <id>`: show|delete DigitalOcean droplet(s)\n" +
			":closed_lock_with_key: `vpn[c|s|d]`: [C]onnect, [S]tatus, [D]rop VPN tunnel to Fritz!Box\n" +
			":pirate_bay: `torq <search term>`\n" +
			":openvpn: `ovpn`: show status of PrivateTunnel on :raspberry_pi:\n" +
			":transmission: `tran[c|s|d]`: [C]reate <URL>, [S]tatus, [D]elete <ID> torrents on :raspberry_pi:\n" +
			":floppy_disk: `fsck`: show disk space on :raspberry_pi:\n" +
			":recycle: `rm(|mv) <filename>` from :raspberry_pi: (to `" + piUSBMountPath + "`)\n" +
			":baseball: `bb`: show yesterday's baseball games\n" +
			":youtube: `yt <video url>`: Download Youtube video to Papa's handy\n"
		api.PostMessage(slackMessage.Channel, response, params)
	} else {
		rtm.SendMessage(rtm.NewOutgoingMessage("whaddya say <@"+callingUserProfile.Name+">? Try `help` instead...",
			slackMessage.Channel))
	}
}

func mvvRoute(origin string, destination string) string {
	loc, _ := time.LoadLocation("Europe/Berlin")
	date := time.Now().In(loc)

	yearObj := date.Year()
	monthObj := int(date.Month())
	dayObj := date.Day()
	hourObj := date.Hour()
	minuteObj := date.Minute()

	month := strconv.Itoa(monthObj)
	hour := strconv.Itoa(hourObj)
	day := strconv.Itoa(dayObj)
	minute := strconv.Itoa(minuteObj)
	year := strconv.Itoa(yearObj)

	return "http://efa.mvv-muenchen.de/mvv/XSLT_TRIP_REQUEST2?&language=de" +
		"&anyObjFilter_origin=0&sessionID=0&itdTripDateTimeDepArr=dep&type_destination=any" +
		"&itdDateMonth=" + month + "&itdTimeHour=" + hour + "&anySigWhenPerfectNoOtherMatches=1" +
		"&locationServerActive=1&name_origin=" + origin + "&itdDateDay=" + day + "&type_origin=any" +
		"&name_destination=" + destination + "&itdTimeMinute=" + minute + "&Session=0&stateless=1" +
		"&SpEncId=0&itdDateYear=" + year
}

func sendPayloadToJoinAPI(downloadFilename string) string {
	response := "Sorry, couldn't download URL..."

	vid, _ := ytdl.GetVideoInfo(downloadFilename)

	// NOW send this URL to the Join Push App API
	pushURL := "https://joinjoaomgcd.appspot.com/_ah/api/messaging/v1/sendPush"
	defaultParams := "?deviceId=007e5b72192c420d9115334d1f177c4c&icon=https://emoji.slack-edge.com/T092UA8PR/youtube/a9a89483b7536f8a.png&smallicon=https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcR1IVXeyHHqhrZF48iK4bxzAjy3vlDoW9nVvTQoEL-tjOXygr-GWQ"
	fileOnPhone := "&title=" + vid.Title
	fileURL := "&file=" + downloadFilename
	apiKey := "&apikey=" + joinAPIKey

	completeURL := pushURL + defaultParams + fileOnPhone + fileURL + apiKey
	// Get the data
	log.Printf("joinPushURL: %s\n", completeURL)
	resp, err := http.Get(completeURL)
	if err != nil {
		log.Printf("ERR: unable to call Join Push")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		response = "Sending '" + vid.Title + "' to Papa's handy..."
	}

	return response
}

/* DownloadFile is now exported
func DownloadFile(search string) {
	torrents, results := SearchFor(search, 200)
	for num, torrent := range torrents {
		if num < 20 {
			fmt.Println(torrent.Title)
			// TODO figure out date of game and compare to today's date
			// type1: NFL.2016.RS.W12.(28 nov).GB
			// type2: NFL.2016.12.11.Cowboys
			// type3: NFL.2016.RS.W13.KC.

		}
	}

	var tor []string
	tor[0] = tranc
	tor[1] = results
	if runningFritzboxTunnel() {
		trans := torrentCommand(tor)
		fmt.Println(trans)
	}
}*/
