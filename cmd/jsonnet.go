package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/apptio/kr8/internal/util"
	goyaml "github.com/ghodss/yaml"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	pruneFlag       bool
	outputFormat    string
	extVarFileFlag  []string
	jsonnetIncludes []string
)

// Takes a list of jsonnet files and imports each one and mixes them with "+"
func renderJsonnet(cmd *cobra.Command, files []string, param string, prune bool, prepend string, source string) string {

	// copy the slice so that we don't unitentionally modify the original
	jsonnetPaths := make([]string, len(files[:0]))
	copy(jsonnetPaths, files[:0])

	// range through the files
	for _, s := range files {
		jsonnetPaths = append(jsonnetPaths, fmt.Sprintf("(import '%s')", s))
	}

	// Join the slices into a jsonnet compat string. Prepend code from "prepend" variable, if set.
	var jsonnetImport string
	if prepend != "" {
		jsonnetImport = prepend + "+" + strings.Join(jsonnetPaths, "+")
	} else {
		jsonnetImport = strings.Join(jsonnetPaths, "+")
	}

	if param != "" {
		jsonnetImport = "(" + jsonnetImport + ")" + param
	}

	if prune {
		// wrap in std.prune, to remove nulls, empty arrays and hashes
		jsonnetImport = "std.prune(" + jsonnetImport + ")"
	}

	// Create a JSonnet VM
	vm, err := util.JsonnetVM(cmd, baseDir)
	if err != nil {
		log.Fatal().Err(err).Msg("Error creating jsonnet VM")
	}
	// render the jsonnet
	out, err := vm.EvaluateAnonymousSnippet(source, jsonnetImport)

	if err != nil {
		log.Fatal().Err(err).Msg("Error evaluating jsonnet snippet")
	}

	return out

}

var jsonnetCmd = &cobra.Command{
	Use:   "jsonnet",
	Short: "Jsonnet utilities",
	Long:  `Utility commands to process jsonnet`,
}

var jsonnetrenderCmd = &cobra.Command{
	Use:   "render file [file ...]",
	Short: "Render a jsonnet file",
	Long:  `Render a jsonnet file to JSON or YAML`,

	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clusterName := viper.GetString("cluster")

		if clusterName == "" && clusterParams == "" {
			log.Fatal().Msg("Please specify a --cluster name and/or --clusterparams")
		}

		config := renderClusterParams(cmd, clusterName, []string{componentName}, clusterParams, false)

		// VM
		vm, _ := util.JsonnetVM(cmd, baseDir)

		var input string
		// pass component, _cluster and _components as extvars

		vm.ExtCode("kr8_cluster", "std.prune("+config+"._cluster)")
		vm.ExtCode("kr8_components", "std.prune("+config+"._components)")
		vm.ExtCode("kr8", "std.prune("+config+"."+componentName+")")
		vm.ExtCode("kr8_unpruned", config+"."+componentName)

		if pruneFlag {
			input = "std.prune(import '" + args[0] + "')"
		} else {
			input = "( import '" + args[0] + "')"
		}
		j, err := vm.EvaluateAnonymousSnippet("file", input)

		if err != nil {
			log.Fatal().Err(err).Msg("Error evaluating jsonnet snippet")
		}
		switch outputFormat {
		case "yaml":
			yaml, err := goyaml.JSONToYAML([]byte(j))
			if err != nil {
				log.Fatal().Err(err).Msg("Error converting JSON to YAML")
			}
			fmt.Println(string(yaml))
		case "stream": // output yaml stream
			var o []interface{}
			if err := json.Unmarshal([]byte(j), &o); err != nil {
				log.Fatal().Err(err).Msg("")
			}
			for _, jobj := range o {
				fmt.Println("---")
				buf, err := goyaml.Marshal(jobj)
				if err != nil {
					log.Fatal().Err(err).Msg("")
				}
				fmt.Println(string(buf))
			}
		case "json":
			formatted := Pretty(j, colorOutput)
			fmt.Println(formatted)
		default:
			log.Fatal().Msg("Output format must be json, yaml or stream")
		}
	},
}

func init() {
	RootCmd.AddCommand(jsonnetCmd)
	jsonnetCmd.AddCommand(jsonnetrenderCmd)
	jsonnetrenderCmd.PersistentFlags().BoolVarP(&pruneFlag, "prune", "", true, "Prune null and empty objects from rendered json")
	jsonnetrenderCmd.PersistentFlags().StringVarP(&clusterParams, "clusterparams", "", "", "provide cluster params as single file - can be combined with --cluster to override cluster")
	jsonnetrenderCmd.PersistentFlags().StringVarP(&componentName, "component", "C", "", "component to render params for")
	jsonnetrenderCmd.PersistentFlags().StringVarP(&outputFormat, "format", "F", "json", "Output forma: json, yaml, stream")

	jsonnetrenderCmd.PersistentFlags().StringP("cluster", "c", "", "cluster to render params for")
	viper.BindPFlag("cluster", jsonnetrenderCmd.PersistentFlags().Lookup("cluster"))
}
