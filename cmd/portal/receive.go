package main

import (
	"fmt"
	"os"

	"github.com/SpatiumPortae/portal/internal/file"
	"github.com/SpatiumPortae/portal/internal/password"
	"github.com/SpatiumPortae/portal/internal/semver"
	"github.com/SpatiumPortae/portal/ui/receiver"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// receiveCmd is the cobra command for `portal receive`
var receiveCmd = &cobra.Command{
	Use:   "receive",
	Short: "Receive files",
	Long:  "The receive command receives files from the sender with the matching password.",
	Args:  cobra.ExactArgs(1),
	PreRun: func(cmd *cobra.Command, args []string) {
		// Bind flags to viper
		//nolint
		viper.BindPFlag("rendezvousPort", cmd.Flags().Lookup("rendezvous-port"))
		//nolint
		viper.BindPFlag("rendezvousAddress", cmd.Flags().Lookup("rendezvous-address"))
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		file.RemoveTemporaryFiles(file.RECEIVE_TEMP_FILE_NAME_PREFIX)
		err := validateRendezvousAddressInViper()
		if err != nil {
			return err
		}
		logFile, err := setupLoggingFromViper("receive")
		if err != nil {
			return err
		}
		defer logFile.Close()
		pwd := args[0]
		if !password.IsValid(pwd) {
			return fmt.Errorf("invalid password format")
		}
		handleReceiveCommand(pwd)
		return nil
	},
}

// Setup flags
func init() {
	// Add subcommand flags (dummy default values as default values are handled through viper)
	//TODO: recactor this into a single flag for providing a TCPAddr
	receiveCmd.Flags().IntP("rendezvous-port", "p", 0, "port on which the rendezvous server is running")
	receiveCmd.Flags().StringP("rendezvous-address", "a", "", "host address for the rendezvous server")
}

// handleReceiveCommand is the receive application.
func handleReceiveCommand(password string) {
	addr := viper.GetString("rendezvousAddress")
	port := viper.GetInt("rendezvousPort")
	var opts []receiver.Option
	ver, err := semver.Parse(version)
	if err == nil {
		opts = append(opts, receiver.WithVersion(ver))
	}
	receiver := receiver.New(fmt.Sprintf("%s:%d", addr, port), password, opts...)

	if err := receiver.Start(); err != nil {
		fmt.Println("Error initializing UI", err)
		os.Exit(1)
	}
	fmt.Println("")
	os.Exit(0)
}
