package cmd

import (
	"fmt"
	"io/ioutil"

	"github.com/fllaca/okay/pkg/fs"
	"github.com/fllaca/okay/pkg/kubernetes"
	"github.com/fllaca/okay/pkg/utils"
	"github.com/fllaca/okay/pkg/validate"
	"github.com/spf13/cobra"

	"github.com/gookit/color"
)

var (
	rootCmd = &cobra.Command{
		Use:   "okay",
		Short: "A Kubernetes manifests validator tool",
		Long:  `A Kubernetes manifests validator tool`,
		Run: func(cmd *cobra.Command, args []string) {
			// TODO: calculate exit code
			runValidate(filenames, openApiSchema, crds, recursive)
		},
	}
	filenames     = make([]string, 0)
	crds          = make([]string, 0)
	openApiSchema = ""
	recursive     = false
)

func init() {
	rootCmd.PersistentFlags().StringArrayVarP(&filenames, "filename", "f", []string{}, "(required) file or directories that contain the configuration to be validated")
	// TODO support OpenApi V3 input
	rootCmd.PersistentFlags().StringVarP(&openApiSchema, "schema", "s", "", "(required) Kubernetes OpenAPI V2 schema to validate against")
	rootCmd.PersistentFlags().BoolVarP(&recursive, "recursive", "R", false, "process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.")
	rootCmd.PersistentFlags().StringArrayVarP(&crds, "crd", "c", []string{}, "files or directories that contain CustomResourceDefinitions to be used for validation")
	rootCmd.MarkPersistentFlagRequired("filename")
	rootCmd.MarkPersistentFlagRequired("schema")
}

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func runValidate(filenames []string, schema string, crds []string, recursive bool) {
	fmt.Printf("Validating config in %s against schema in %s\n", utils.JoinNotEmptyStrings(", ", filenames...), openApiSchema)

	opeanApi2SpecsBytes, err := ioutil.ReadFile(openApiSchema)
	if err != nil {
		fmt.Printf("Error loading specs from %s: %s\n", openApiSchema, err)
		return
	}
	resourceValidator, err := validate.NewOpenApi2Validator(opeanApi2SpecsBytes)
	if err != nil {
		fmt.Printf("Error loading specs from %s: %s\n", openApiSchema, err)
		return
	}

	for _, crd := range crds {
		err := fs.ApplyToPathWithFilter(crd, false, func(file string) error {
			fmt.Printf("Using CustomResourceDefinitions from %s\n", file)
			fileBytes, err := ioutil.ReadFile(file)
			if err != nil {
				return err
			}
			crdResources, err := kubernetes.ParseResourcesFromYaml(fileBytes)
			if err != nil {
				return err
			}
			for _, crdResource := range crdResources {
				err = resourceValidator.AddCrdSchemas(crdResource)
				if err != nil {
					return err
				}
			}
			return nil
		}, fs.IsYamlFilter)
		if err != nil {
			fmt.Printf("Error loading CustomResourceDefinitions from %s: %s\n", crd, err)
		}
	}

	fileValidator := validate.NewYamlFileValidator(resourceValidator)

	fmt.Println("Results:")
	for _, filename := range filenames {
		err := fs.ApplyToPathWithFilter(filename, recursive, func(file string) error {
			fmt.Printf("Validating manifests in %s:\n", file)
			fileBytes, err := ioutil.ReadFile(file)
			if err != nil {
				fmt.Printf("Error reading file %s: %s\n", file, err)
				// continue processing other files in input
				return nil
			}

			validationResults := fileValidator.Validate(fileBytes)
			outputResult(validationResults)
			return nil

		}, fs.IsYamlFilter)
		if err != nil {
			fmt.Printf("Error while validating %s: %s\n", filename, err)
		}
	}
}

func outputResult(results []validate.ValidationResult) {
	for _, result := range results {
		fmt.Printf("\t - %s, %s (%s): %s\n", colorSeverity(result.Severity), utils.JoinNotEmptyStrings("/", result.Namespace, result.Name), result.Kind, result.Message)
	}
	fmt.Println()
}

func colorSeverity(severity validate.Severity) string {
	red := color.FgRed.Render
	green := color.FgGreen.Render
	yellow := color.FgYellow.Render
	switch severity {
	case validate.SeverityError:
		return red(severity)
	case validate.SeverityWarning:
		return yellow(severity)
	case validate.SeverityOK:
		return green(severity)
	default:
		return (string)(severity)
	}
}
