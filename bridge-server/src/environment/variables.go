package environment

// external packages
import godotenv "github.com/joho/godotenv"

///////////////////////
//   get variables   //
///////////////////////

func getEnvironmentVariables(variableName string) string {
	myEnvironmentVariable, _ := godotenv.Read()
	return myEnvironmentVariable[variableName]
}

/////////////////////
//  set variables  //
/////////////////////

var (
	PORT                    = getEnvironmentVariables("PORT")
	DATABASE_URL            = getEnvironmentVariables("DATABASE_URL")
	BRIDGE_DATABASE_URL     = getEnvironmentVariables("BRIDGE_DATABASE_URL")
	BRIDGE_DATABASE_DIALECT = getEnvironmentVariables("BRIDGE_DATABASE_DIALECT")
)
