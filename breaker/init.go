package breaker

import (
	"log"
	"os"
	"path"
)

// InitBreaker initializes a new BreakerAPI instance.
// It attempts to load the circuit breaker's configuration from the file specified by pathToConfig.
//
// Parameters:
//
//	pathToConfig: A string representing the absolute or relative path to the
//	              circuit breaker's configuration file (e.g., "config/breaker.toml").
//	dftConfig:    A pointer to a Config struct containing default configuration values.
//	              This default configuration is used if the specified configuration file
//	              does not exist, cannot be read, or is found to be corrupted.
//
// Returns:
//
//	A pointer to an initialized BreakerAPI struct. The BreakerAPI will be configured
//	based on the following priority:
//	1. Successfully loaded configuration from pathToConfig.
//	2. Default configuration (dftConfig) if pathToConfig does not exist (in which case,
//	   the default configuration is also saved to pathToConfig).
//	3. Default configuration (dftConfig) if pathToConfig exists but is corrupted or
//	   cannot be loaded (in which case, an attempt is made to overwrite
//	   pathToConfig with dftConfig).
//	4. Default configuration (dftConfig) used in-memory if any critical file/directory
//	   operations fail (e.g., cannot create directory, cannot save default config).
//	   In such cases, the BreakerAPI's Driver will still be associated with the
//	   original pathToConfig.
//
// Behavior:
//  1. Checks if the configuration file at `pathToConfig` exists.
//     a. If the file does NOT exist:
//     i. Logs an attempt to create it.
//     ii. Tries to create the necessary directory structure for `pathToConfig`.
//     - If directory creation fails, logs the error and returns a BreakerAPI
//     initialized with `dftConfig` (in-memory), but its Driver will still
//     reference `pathToConfig`.
//     iii. Tries to save `dftConfig` to `pathToConfig`.
//     - If saving fails, logs the error and returns a BreakerAPI initialized
//     with `dftConfig` (in-memory), with its Driver referencing `pathToConfig`.
//     iv. If successful, logs the creation and returns a BreakerAPI initialized
//     with `dftConfig`, using the newly created file at `pathToConfig`.
//     b. If there's an error (other than "not exist") checking the file (e.g., permission issues):
//     i. Logs the error.
//     ii. Proceeds to attempt loading the configuration anyway (step 2).
//     c. If the file exists:
//     i. Logs that the file was found, along with its size and permissions.
//
//  2. Attempts to load the configuration from `pathToConfig` using `LoadConfig`.
//     a. If `LoadConfig` fails (e.g., file is corrupted, parsing error):
//     i. Logs the error.
//     ii. Sets the active configuration to `dftConfig`.
//     iii. If the file was known to exist initially (i.e., `os.Stat` was successful),
//     it attempts to overwrite the (potentially corrupted) `pathToConfig`
//     with `dftConfig`. Logs success or failure of this save operation.
//     b. If `LoadConfig` succeeds:
//     i. The loaded configuration is used.
//
// 3. Creates and returns a `BreakerAPI` instance.
//   - The `Config` field of `BreakerAPI` is populated with the determined configuration
//     (either loaded or default).
//   - The `Driver` field is initialized by calling `NewBreaker` with the determined
//     configuration and the original `pathToConfig`.
//   - Logs the successful creation and the final configuration values.
func InitBreaker(pathToConfig string, dftConfig *Config) *BreakerAPI {

	// Check if the file exists
	fileInfo, err := os.Stat(pathToConfig)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Breaker config not found at %s, will attempt to create it", pathToConfig)

			// Create the directory if it doesn't exist
			err := os.MkdirAll(path.Dir(pathToConfig), os.ModePerm)
			if err != nil {
				log.Printf("Failed to create directory: %v, using default config in memory only", err)
				// In the case of failure, also use the right path
				return &BreakerAPI{
					Config: *dftConfig,
					Driver: NewBreaker(dftConfig, pathToConfig),
				}
			}

			// Save default config file after creating directory
			err = SaveConfig(pathToConfig, dftConfig)
			if err != nil {
				log.Printf("Failed to save default config: %v, using default config in memory only", err)
				// In the case of failure, also use the right path
				return &BreakerAPI{
					Config: *dftConfig,
					Driver: NewBreaker(dftConfig, pathToConfig),
				}
			}

			log.Printf("Default breaker config saved to %s", pathToConfig)

			// Create BreakerAPI with the default configuration and the correct path
			retVal := &BreakerAPI{
				Config: *dftConfig,
				Driver: NewBreaker(dftConfig, pathToConfig),
			}

			log.Printf("BreakerAPI created with default config using path: %s", pathToConfig)
			return retVal

		} else {
			log.Printf("Error when checking the file: %v", err)
			// Continue execution to try to load the config anyway
		}
	} else {
		log.Printf("File found - Size: %d bytes, Permissions: %s", fileInfo.Size(), fileInfo.Mode())
	}

	// At this point, either the file existed, or we had an error but want to try loading anyway
	// Attempt to load the configuration
	log.Printf("Attempting to load configuration from %s", pathToConfig)
	config, err := LoadConfig(pathToConfig)
	if err != nil {
		log.Printf("Error in LoadConfig(): %v", err)
		log.Printf("Failed to load breaker config: %v, using default config", err)

		// If the load fails, use the default configuration and save it
		config = dftConfig

		// Try to save the default settings (only if the file existed and was corrupt)
		if fileInfo != nil { // fileInfo will be nil if os.Stat failed with an error other than IsNotExist
			log.Printf("Attempting to save default config as the existing file might be corrupted")
			err = SaveConfig(pathToConfig, config)
			if err != nil {
				log.Printf("Failed to save default config: %v", err)
			} else {
				log.Printf("Default config saved to %s", pathToConfig)
			}
		}
	}

	// Create BreakerAPI manually with the right path
	retVal := &BreakerAPI{
		Config: *config,
		Driver: NewBreaker(config, pathToConfig),
	}

	log.Printf("Successfully created BreakerAPI using config file: %s", pathToConfig)
	log.Printf("Loaded Breaker Config: %+v", *config)

	return retVal
}
