package conn

import (
	"context"
	"encoding/base64"
	"fmt"
	"hisoka/src/handlers"
	"hisoka/src/helpers"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	_ "hisoka/src/commands"
	_ "github.com/mattn/go-sqlite3"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

type Template struct {
	Nama   string
	Status bool
	QRCode string
}

var log helpers.Logger

func init() {
	store.DeviceProps.PlatformType = waCompanionReg.DeviceProps_EDGE.Enum()
	store.DeviceProps.Os = proto.String("Linux")
}

const htmlTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>WhatsApp QR Code</title>
    <style>
        body {
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            height: 100vh;
            margin: 0;
            background-color: #f0f2f5;
            font-family: Arial, sans-serif;
        }
        .qr-container {
            background-color: white;
            padding: 20px;
            border-radius: 10px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        h1 {
            color: #128C7E;
            margin-bottom: 20px;
        }
        .status {
            margin-top: 20px;
            color: #666;
        }
        img {
            max-width: 300px;
            height: auto;
        }
    </style>
</head>
<body>
    <div class="qr-container">
        <h1>WhatsApp QR Code</h1>
        {{if .QRCode}}
            <img src="data:image/png;base64,{{.QRCode}}" alt="QR Code">
            <p class="status">Scan this QR code with WhatsApp on your phone</p>
        {{else}}
            <p class="status">Waiting for QR code...</p>
        {{end}}
    </div>
</body>
</html>
`

func generateQRCodeImage(code string) (string, error) {
	qr, err := qrcode.New(code, qrcode.Medium)
	if err != nil {
		return "", err
	}

	png, err := qr.PNG(256)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(png), nil
}

func StartClient() {
	dbLog := waLog.Stdout("Database", "ERROR", true)
	container, err := sqlstore.New("sqlite3", "file:session.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}

	handler := handlers.NewHandler(container)
	log.Info("Connecting Socket")
	conn := handler.Client()

	conn.PrePairCallback = func(jid types.JID, platform, businessName string) bool {
		log.Info("Connected Socket")
		return true
	}

	if conn.Store.ID == nil {
		// No ID stored, new login
		pairingNumber := os.Getenv("PAIRING_NUMBER")
		if pairingNumber != "" {
			pairingNumber = regexp.MustCompile(`\D+`).ReplaceAllString(pairingNumber, "")
			if err := conn.Connect(); err != nil {
				panic(err)
			}
			code, err := conn.PairPhone(pairingNumber, true, whatsmeow.PairClientChrome, "Edge (Linux)")
			if err != nil {
				panic(err)
			}
			fmt.Println("Code Kamu : " + code)
		} else {
			// Create channels for QR code updates
			qrChan, _ := conn.GetQRChannel(context.Background())
			
			// Start HTTP server
			tmpl := template.Must(template.New("qr").Parse(htmlTemplate))
			data := &Template{
				Nama:   "WhatsApp QR",
				Status: false,
			}

			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				tmpl.Execute(w, data)
			})

			// Start server in a goroutine
			go func() {
				log.Info("Starting server at http://localhost:8080")
				if err := http.ListenAndServe(":8080", nil); err != nil {
					// log.Error("HTTP server error:", err)
				}
			}()

			if err := conn.Connect(); err != nil {
				panic(err)
			}

			for evt := range qrChan {
				switch string(evt.Event) {
				case "code":
					qrImage, err := generateQRCodeImage(evt.Code)
					if err != nil {
						// log.Error("Failed to generate QR code:", err)
						continue
					}
					data.QRCode = qrImage
					data.Status = true
					log.Info("New QR code generated - please refresh the page")
				}
			}
		}
	} else {
		// Already logged in, just connect
		if err := conn.Connect(); err != nil {
			panic(err)
		}
		log.Info("Connected Socket")
	}

	// Listen to Ctrl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	conn.Disconnect()
}
