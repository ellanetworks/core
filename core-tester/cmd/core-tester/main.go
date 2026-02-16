package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/ellanetworks/core/core-tester/internal/register"
	"github.com/ellanetworks/core/core-tester/internal/tests/engine"
	"github.com/ellanetworks/core/core-tester/internal/tests/tests"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	outputFile            string
	imsi                  string
	key                   string
	opc                   string
	sqn                   string
	policyName            string
	mcc                   string
	mnc                   string
	sst                   int32
	sd                    string
	tac                   string
	dnn                   string
	gnbN2Address          string
	gnbN3Address          string
	gnbN3AddressSecondary string
	ellaCoreN2Address     string
	ellaCoreAPIAddress    string
	ellaCoreAPIToken      string
	testInclude           []string
	testExclude           []string
	verbose               bool
	pingDestination       string
)

var rootCmd = &cobra.Command{
	Use:   "ella-core-tester [command]",
	Short: "A tool for testing Ella Core",
	Long:  `Ella Core Tester validates functionality, connectivity, and performance.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbose {
			logger.Init(zap.DebugLevel)
		} else {
			logger.Init(zap.InfoLevel)
		}
	},
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run the complete test suite",
	Long:  "Run all registered tests against the Ella Core instance. This procedure is intrusive and will create and delete resources in Ella Core.",
	Args:  cobra.NoArgs,
	Run:   Test,
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a subscriber in Ella Core and create a GTP tunnel",
	Long:  "Register a subscriber in Ella Core and create a GTP tunnel. The subscriber needs to already be created in Ella Core. This procedure will not try to create and delete resources in Ela Core.",
	Args:  cobra.NoArgs,
	Run:   Register,
}

func main() {
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(registerCmd)
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose (debug) logging")

	testCmd.Flags().StringVar(&ellaCoreAPIAddress, "ella-core-api-address", "", "Ella Core API address")
	testCmd.Flags().StringVar(&ellaCoreAPIToken, "ella-core-api-token", "", "Ella Core API token")
	testCmd.Flags().StringVar(&ellaCoreN2Address, "ella-core-n2-address", "", "Ella Core N2 address")
	testCmd.Flags().StringVar(&gnbN2Address, "gnb-n2-address", "", "gNB N2 address")
	testCmd.Flags().StringVar(&gnbN3Address, "gnb-n3-address", "", "gNB N3 address")
	testCmd.Flags().StringVar(&gnbN3AddressSecondary, "gnb-n3-address-secondary", "", "Secondary gNB N3 address (for Xn handover target gNB)")
	testCmd.Flags().StringSliceVarP(
		&testInclude,
		"include",
		"i",
		[]string{},
		"Substring to include tests by ID, can be specified multiple times",
	)
	testCmd.Flags().StringSliceVarP(
		&testExclude,
		"exclude",
		"e",
		[]string{},
		"Substring to exclude tests by ID, can be specified multiple times",
	)
	testCmd.Flags().StringVarP(&outputFile, "write", "w", "", "Write test results (JSON) to file")
	testCmd.Flags().StringVar(&pingDestination, "ping-destination", "10.6.0.3", "Destination to ping when testing UE connectivity")
	_ = testCmd.MarkFlagRequired("ella-core-api-address")
	_ = testCmd.MarkFlagRequired("ella-core-api-token")
	_ = testCmd.MarkFlagRequired("ella-core-n2-address")
	_ = testCmd.MarkFlagRequired("gnb-n2-address")
	_ = testCmd.MarkFlagRequired("gnb-n3-address")

	registerCmd.Flags().StringVar(&imsi, "imsi", "", "IMSI of the subscriber")
	registerCmd.Flags().StringVar(&key, "key", "", "Key of the subscriber")
	registerCmd.Flags().StringVar(&opc, "opc", "", "OPC of the subscriber")
	registerCmd.Flags().StringVar(&sqn, "sqn", "", "SQN of the subscriber")
	registerCmd.Flags().StringVar(&policyName, "policy-name", "", "Policy name of the subscriber")
	registerCmd.Flags().StringVar(&mcc, "mcc", "", "MCC of the subscriber")
	registerCmd.Flags().StringVar(&mnc, "mnc", "", "MNC of the subscriber")
	registerCmd.Flags().Int32Var(&sst, "sst", 0, "SST of the subscriber")
	registerCmd.Flags().StringVar(&sd, "sd", "", "SD of the subscriber")
	registerCmd.Flags().StringVar(&tac, "tac", "", "TAC of the subscriber")
	registerCmd.Flags().StringVar(&dnn, "dnn", "dnn", "DNN of the subscriber")
	registerCmd.Flags().StringVar(&gnbN2Address, "gnb-n2-address", "", "gNB N2 address")
	registerCmd.Flags().StringVar(&gnbN3Address, "gnb-n3-address", "", "gNB N3 address")
	registerCmd.Flags().StringVar(&ellaCoreN2Address, "ella-core-n2-address", "", "Ella Core N2 address")
	_ = registerCmd.MarkFlagRequired("imsi")
	_ = registerCmd.MarkFlagRequired("key")
	_ = registerCmd.MarkFlagRequired("opc")
	_ = registerCmd.MarkFlagRequired("sqn")
	_ = registerCmd.MarkFlagRequired("policy-name")
	_ = registerCmd.MarkFlagRequired("mcc")
	_ = registerCmd.MarkFlagRequired("mnc")
	_ = registerCmd.MarkFlagRequired("sst")
	_ = registerCmd.MarkFlagRequired("tac")
	_ = registerCmd.MarkFlagRequired("dnn")
	_ = registerCmd.MarkFlagRequired("gnb-n2-address")
	_ = registerCmd.MarkFlagRequired("gnb-n3-address")
	_ = registerCmd.MarkFlagRequired("ella-core-n2-address")

	rootCmd.CompletionOptions.DisableDefaultCmd = true

	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func Test(cmd *cobra.Command, args []string) {
	testConfig := engine.Config{
		PingDestination: pingDestination,
		EllaCore: engine.EllaCoreConfig{
			N2Address: ellaCoreN2Address,
		},
		Gnb: engine.GnbConfig{
			N2Address:          gnbN2Address,
			N3Address:          gnbN3Address,
			N3AddressSecondary: gnbN3AddressSecondary,
		},
	}

	err := tests.RegisterAll(testInclude, testExclude)
	if err != nil {
		logger.Logger.Fatal("Could not register tests", zap.Error(err))
	}

	clientConfig := &client.Config{
		BaseURL:  ellaCoreAPIAddress,
		APIToken: ellaCoreAPIToken,
	}

	ellaClient, err := client.New(clientConfig)
	if err != nil {
		logger.Logger.Fatal("failed to create ella client", zap.Error(err))
	}

	testEnv := engine.Env{
		Config:         testConfig,
		EllaCoreClient: ellaClient,
	}

	allPassed, testResults := engine.Run(context.Background(), testEnv)

	err = writeResultsToFile(outputFile, testResults)
	if err != nil {
		logger.Logger.Fatal("Could not write test results to file", zap.Error(err))
	}

	if !allPassed {
		logger.Logger.Fatal("Some tests failed")
	}
}

func Register(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	registerConfig := register.RegisterConfig{
		IMSI:              imsi,
		Key:               key,
		OPC:               opc,
		SequenceNumber:    sqn,
		PolicyName:        policyName,
		MCC:               mcc,
		MNC:               mnc,
		SST:               sst,
		SD:                sd,
		TAC:               tac,
		DNN:               dnn,
		GnbN2Address:      gnbN2Address,
		GnbN3Address:      gnbN3Address,
		EllaCoreN2Address: ellaCoreN2Address,
	}

	err := register.Register(ctx, registerConfig)
	if err != nil {
		logger.Logger.Fatal("Could not register", zap.Error(err))
	}
}

func writeResultsToFile(filePath string, testResults []engine.TestResult) error {
	if filePath == "" {
		return nil
	}

	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("could not create output file: %v", err)
	}

	defer func() {
		_ = f.Close()
	}()

	b, err := json.Marshal(testResults)
	if err != nil {
		return fmt.Errorf("could not marshal test results to json: %v", err)
	}

	_, err = f.Write(b)
	if err != nil {
		return fmt.Errorf("could not write test results to file: %v", err)
	}

	return nil
}
