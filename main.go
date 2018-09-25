package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"encoding/json"

	"github.com/asticode/go-astilectron"
	"github.com/asticode/go-astilectron-bootstrap"
	"github.com/asticode/go-astilog"
	"github.com/mit-dci/lit/crypto/koblitz"
	"github.com/mit-dci/lit/litrpc"
	"github.com/mit-dci/lit/lnutil"
	"github.com/pkg/errors"
)

// Constants
const htmlAbout = `Welcome on <b>Astilectron</b> demo!<br>
This is using the bootstrap and the bundler.`

// Vars
var (
	AppName string
	BuiltAt string
	debug   = flag.Bool("d", false, "enables the debug mode")
	conptr  = flag.String("con", "@:2448", "host to connect to in the form of [<lnadr>@][<host>][:<port>]")
	dirptr  = flag.String("dir", filepath.Join(os.Getenv("HOME"), ".lit"), "directory to save settings")
	w       *astilectron.Window
)

func main() {
	// Init
	flag.Parse()
	astilog.FlagInit()

	initProxy(*conptr, *dirptr)

	// Run bootstrap
	astilog.Debugf("Running app built at %s", BuiltAt)
	if err := bootstrap.Run(bootstrap.Options{
		AstilectronOptions: astilectron.Options{
			AppName:            AppName,
			AppIconDarwinPath:  "resources/icon.icns",
			AppIconDefaultPath: "resources/icon.png",
		},
		Debug: *debug,
		MenuOptions: []*astilectron.MenuItemOptions{{
			Label: astilectron.PtrStr("File"),
			SubMenu: []*astilectron.MenuItemOptions{
				{
					Label: astilectron.PtrStr("About"),
					OnClick: func(e astilectron.Event) (deleteListener bool) {
						if err := bootstrap.SendMessage(w, "about", htmlAbout, func(m *bootstrap.MessageIn) {
							// Unmarshal payload
							var s string
							if err := json.Unmarshal(m.Payload, &s); err != nil {
								astilog.Error(errors.Wrap(err, "unmarshaling payload failed"))
								return
							}
							astilog.Infof("About modal has been displayed and payload is %s!", s)
						}); err != nil {
							astilog.Error(errors.Wrap(err, "sending about event failed"))
						}
						return
					},
				},
				{Role: astilectron.MenuItemRoleClose},
			},
		}},
		OnWait: func(_ *astilectron.Astilectron, ws []*astilectron.Window, _ *astilectron.Menu, _ *astilectron.Tray, _ *astilectron.Menu) error {
			w = ws[0]
			go func() {
				time.Sleep(5 * time.Second)
				if err := bootstrap.SendMessage(w, "check.out.menu", "Don't forget to check out the menu!"); err != nil {
					astilog.Error(errors.Wrap(err, "sending check.out.menu event failed"))
				}
			}()
			return nil
		},
		Windows: []*bootstrap.Window{{
			Homepage:       "http://localhost:3000/?port=49586",
			MessageHandler: handleMessages,
			Options: &astilectron.WindowOptions{
				BackgroundColor: astilectron.PtrStr("#333"),
				Center:          astilectron.PtrBool(true),
				Height:          astilectron.PtrInt(700),
				Width:           astilectron.PtrInt(700),
			},
		}},
	}); err != nil {
		astilog.Fatal(errors.Wrap(err, "running bootstrap failed"))
	}
}

func initProxy(con, homeDir string) {
	var lndcRpcClient *litrpc.LndcRpcClient
	var err error

	adr, host, port := lnutil.ParseAdrStringWithPort(con)

	if litrpc.LndcRpcCanConnectLocallyWithHomeDir(homeDir) && adr == "" && (host == "localhost" || host == "127.0.0.1") {

		lndcRpcClient, err = litrpc.NewLocalLndcRpcClientWithHomeDirAndPort(homeDir, port)
		if err != nil {
			log.Fatal(err.Error())
		}
	} else {
		if !lnutil.LitAdrOK(adr) {
			log.Fatal("lit address passed in -con parameter is not valid")
		}

		keyFilePath := filepath.Join(homeDir, "lit-af-key.hex")
		privKey, err := lnutil.ReadKeyFile(keyFilePath)
		if err != nil {
			log.Fatal(err.Error())
		}
		key, pubKey := koblitz.PrivKeyFromBytes(koblitz.S256(), privKey[:])

		adr := fmt.Sprintf("%s@%s:%d", adr, host, port)
		fmt.Printf("Connecting to %s using pubkey %x\n", adr, pubKey.SerializeCompressed())
		lndcRpcClient, err = litrpc.NewLndcRpcClient(adr, key)
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	proxy := litrpc.NewLndcRpcWebsocketProxyWithLndc(lndcRpcClient)
	go proxy.Listen("localhost", 49586)
}
