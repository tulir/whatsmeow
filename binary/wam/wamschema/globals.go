package wamschema

// GENERATED CODE. DO NOT EDIT.

type WAMGlobals struct {
	AbKey2                      *string                                    `wam:"4473,regular"`
	AppBuild                    *AppBuild_AppBuild                         `wam:"1657,regular|private"`
	AppIsBetaRelease            *bool                                      `wam:"21,regular|private"`
	AppVersion                  *string                                    `wam:"17,regular|private"`
	BeaconSessionId             *int32                                     `wam:"18529,regular"`
	Browser                     *string                                    `wam:"779,regular"`
	BrowserVersion              *string                                    `wam:"295,regular"`
	Datacenter                  *string                                    `wam:"2795,regular"`
	DeviceClassification        *DeviceClassification_DeviceClassification `wam:"14507,regular"`
	DeviceName                  *string                                    `wam:"13,regular|private"`
	DeviceVersion               *string                                    `wam:"4505,regular"`
	ExpoKey                     *string                                    `wam:"5029,regular|private"`
	IsInCohort                  *bool                                      `wam:"19129,regular"`
	Mcc                         *int32                                     `wam:"5,regular|private"`
	MemClass                    *int32                                     `wam:"655,regular|private"`
	Mnc                         *int32                                     `wam:"3,regular|private"`
	NetworkIsWifi               *bool                                      `wam:"23,regular"`
	NumCpu                      *int32                                     `wam:"10317,regular"`
	OcVersion                   *int32                                     `wam:"6251,regular|private"`
	OsVersion                   *string                                    `wam:"15,regular|private"`
	Platform                    *Platform_Platform                         `wam:"11,regular|private"`
	PsCountryCode               *string                                    `wam:"6833,private"`
	PsId                        *string                                    `wam:"6005,private"`
	ServiceImprovementOptOut    *bool                                      `wam:"13293,regular|private"`
	StreamId                    *int32                                     `wam:"3543,regular|private"`
	WametaLoggerTestFilter      *string                                    `wam:"15881,regular|private"`
	WebcBucket                  *string                                    `wam:"875,regular"`
	WebcEnv                     *WebcEnv_WebcEnv                           `wam:"633,regular"`
	WebcNativeAutolaunch        *bool                                      `wam:"1009,regular"`
	WebcNativeBetaUpdates       *bool                                      `wam:"1007,regular"`
	WebcPhoneAppVersion         *string                                    `wam:"1005,regular"`
	WebcPhoneCharging           *bool                                      `wam:"783,regular"`
	WebcPhoneDeviceManufacturer *string                                    `wam:"829,regular"`
	WebcPhoneDeviceModel        *string                                    `wam:"831,regular"`
	WebcPhoneOsBuildNumber      *string                                    `wam:"833,regular"`
	WebcPhoneOsVersion          *string                                    `wam:"835,regular"`
	WebcPhonePlatform           *WebcPhonePlatform_WebcPhonePlatform       `wam:"707,regular"`
	WebcRevision                *int32                                     `wam:"18491,regular"`
	WebcTabId                   *string                                    `wam:"3727,regular"`
	WebcWebArch                 *string                                    `wam:"6605,regular"`
	WebcWebDeviceManufacturer   *string                                    `wam:"6599,regular"`
	WebcWebDeviceModel          *string                                    `wam:"6601,regular"`
	WebcWebOsReleaseNumber      *string                                    `wam:"6603,regular"`
	WebcWebPlatform             *WebcWebPlatform_WebcWebPlatform           `wam:"899,regular|private"`
	YearClass                   *int32                                     `wam:"689,regular|private"`
	YearClass2016               *int32                                     `wam:"2617,regular|private"`
	CommitTime                  *int32                                     `wam:"47,regular|private"`
	SequenceNumber              *int32                                     `wam:"3433,regular|private"`
}

type AppBuild_AppBuild int32

const (
	AppBuild_AppBuild_DEBUG   AppBuild_AppBuild = 1
	AppBuild_AppBuild_ALPHA   AppBuild_AppBuild = 2
	AppBuild_AppBuild_BETA    AppBuild_AppBuild = 3
	AppBuild_AppBuild_RELEASE AppBuild_AppBuild = 4
)

type DeviceClassification_DeviceClassification int32

const (
	DeviceClassification_DeviceClassification_MOBILE    DeviceClassification_DeviceClassification = 0
	DeviceClassification_DeviceClassification_TABLET    DeviceClassification_DeviceClassification = 1
	DeviceClassification_DeviceClassification_WEARABLES DeviceClassification_DeviceClassification = 2
	DeviceClassification_DeviceClassification_VR        DeviceClassification_DeviceClassification = 3
	DeviceClassification_DeviceClassification_DESKTOP   DeviceClassification_DeviceClassification = 4
	DeviceClassification_DeviceClassification_FOLDABLE  DeviceClassification_DeviceClassification = 5
	DeviceClassification_DeviceClassification_AR_GLASS  DeviceClassification_DeviceClassification = 6
	DeviceClassification_DeviceClassification_UNDEFINED DeviceClassification_DeviceClassification = 100
)

type Platform_Platform int32

const (
	Platform_Platform_IPHONE            Platform_Platform = 1
	Platform_Platform_ANDROID           Platform_Platform = 2
	Platform_Platform_BB                Platform_Platform = 3
	Platform_Platform_BBX               Platform_Platform = 7
	Platform_Platform_S40               Platform_Platform = 4
	Platform_Platform_SYMBIAN           Platform_Platform = 5
	Platform_Platform_WP                Platform_Platform = 6
	Platform_Platform_WEBCLIENT         Platform_Platform = 8
	Platform_Platform_OSMETA            Platform_Platform = 11
	Platform_Platform_ENT               Platform_Platform = 12
	Platform_Platform_SMBA              Platform_Platform = 13
	Platform_Platform_KAIOS             Platform_Platform = 14
	Platform_Platform_SMBI              Platform_Platform = 15
	Platform_Platform_WINDOWS           Platform_Platform = 16
	Platform_Platform_WEB               Platform_Platform = 17
	Platform_Platform_PORTAL            Platform_Platform = 18
	Platform_Platform_BLOKS             Platform_Platform = 19
	Platform_Platform_BLUEA             Platform_Platform = 20
	Platform_Platform_BLUEI             Platform_Platform = 21
	Platform_Platform_FBLITEA           Platform_Platform = 22
	Platform_Platform_GREENA            Platform_Platform = 23
	Platform_Platform_GREENI            Platform_Platform = 24
	Platform_Platform_IGDA              Platform_Platform = 25
	Platform_Platform_IGDI              Platform_Platform = 26
	Platform_Platform_IGLITEA           Platform_Platform = 27
	Platform_Platform_MLITEA            Platform_Platform = 28
	Platform_Platform_MSGRA             Platform_Platform = 29
	Platform_Platform_MSGRI             Platform_Platform = 30
	Platform_Platform_MSGRP             Platform_Platform = 31
	Platform_Platform_MSGRW             Platform_Platform = 32
	Platform_Platform_IGDW              Platform_Platform = 33
	Platform_Platform_PAGE              Platform_Platform = 34
	Platform_Platform_MSGRDM            Platform_Platform = 35
	Platform_Platform_MSGRDW            Platform_Platform = 36
	Platform_Platform_MSGROM            Platform_Platform = 37
	Platform_Platform_MSGROC            Platform_Platform = 38
	Platform_Platform_MSGRM             Platform_Platform = 43
	Platform_Platform_IGDM              Platform_Platform = 44
	Platform_Platform_WEARM             Platform_Platform = 45
	Platform_Platform_CAPI              Platform_Platform = 46
	Platform_Platform_XR                Platform_Platform = 47
	Platform_Platform_MACOS             Platform_Platform = 48
	Platform_Platform_WAMETA_REPL       Platform_Platform = 49
	Platform_Platform_ARDEV             Platform_Platform = 50
	Platform_Platform_WEAROS            Platform_Platform = 51
	Platform_Platform_MSGRVR            Platform_Platform = 52
	Platform_Platform_BLUEW             Platform_Platform = 53
	Platform_Platform_IPHONEWAMETATEST  Platform_Platform = 54
	Platform_Platform_MSGRAR            Platform_Platform = 57
	Platform_Platform_IPAD              Platform_Platform = 58
	Platform_Platform_WAVOIP_CLI        Platform_Platform = 59
	Platform_Platform_MSGRT             Platform_Platform = 60
	Platform_Platform_IGDT              Platform_Platform = 61
	Platform_Platform_ANDROIDWAMETATEST Platform_Platform = 62
	Platform_Platform_MSGRSG            Platform_Platform = 63
	Platform_Platform_IGDSG             Platform_Platform = 64
	Platform_Platform_INTEROP           Platform_Platform = 65
	Platform_Platform_INTEROP_MSGR      Platform_Platform = 66
	Platform_Platform_IGDVR             Platform_Platform = 67
	Platform_Platform_WASG              Platform_Platform = 68
	Platform_Platform_BLUEVR            Platform_Platform = 69
	Platform_Platform_TEST              Platform_Platform = 9
	Platform_Platform_UNKNOWN           Platform_Platform = 10
)

type WebcEnv_WebcEnv int32

const (
	WebcEnv_WebcEnv_PROD   WebcEnv_WebcEnv = 0
	WebcEnv_WebcEnv_INTERN WebcEnv_WebcEnv = 1
	WebcEnv_WebcEnv_DEV    WebcEnv_WebcEnv = 2
	WebcEnv_WebcEnv_E2E    WebcEnv_WebcEnv = 3
)

type WebcPhonePlatform_WebcPhonePlatform int32

const (
	WebcPhonePlatform_WebcPhonePlatform_IPHONE            WebcPhonePlatform_WebcPhonePlatform = 1
	WebcPhonePlatform_WebcPhonePlatform_ANDROID           WebcPhonePlatform_WebcPhonePlatform = 2
	WebcPhonePlatform_WebcPhonePlatform_BB                WebcPhonePlatform_WebcPhonePlatform = 3
	WebcPhonePlatform_WebcPhonePlatform_BBX               WebcPhonePlatform_WebcPhonePlatform = 7
	WebcPhonePlatform_WebcPhonePlatform_S40               WebcPhonePlatform_WebcPhonePlatform = 4
	WebcPhonePlatform_WebcPhonePlatform_SYMBIAN           WebcPhonePlatform_WebcPhonePlatform = 5
	WebcPhonePlatform_WebcPhonePlatform_WP                WebcPhonePlatform_WebcPhonePlatform = 6
	WebcPhonePlatform_WebcPhonePlatform_WEBCLIENT         WebcPhonePlatform_WebcPhonePlatform = 8
	WebcPhonePlatform_WebcPhonePlatform_OSMETA            WebcPhonePlatform_WebcPhonePlatform = 11
	WebcPhonePlatform_WebcPhonePlatform_ENT               WebcPhonePlatform_WebcPhonePlatform = 12
	WebcPhonePlatform_WebcPhonePlatform_SMBA              WebcPhonePlatform_WebcPhonePlatform = 13
	WebcPhonePlatform_WebcPhonePlatform_KAIOS             WebcPhonePlatform_WebcPhonePlatform = 14
	WebcPhonePlatform_WebcPhonePlatform_SMBI              WebcPhonePlatform_WebcPhonePlatform = 15
	WebcPhonePlatform_WebcPhonePlatform_WINDOWS           WebcPhonePlatform_WebcPhonePlatform = 16
	WebcPhonePlatform_WebcPhonePlatform_WEB               WebcPhonePlatform_WebcPhonePlatform = 17
	WebcPhonePlatform_WebcPhonePlatform_PORTAL            WebcPhonePlatform_WebcPhonePlatform = 18
	WebcPhonePlatform_WebcPhonePlatform_BLOKS             WebcPhonePlatform_WebcPhonePlatform = 19
	WebcPhonePlatform_WebcPhonePlatform_BLUEA             WebcPhonePlatform_WebcPhonePlatform = 20
	WebcPhonePlatform_WebcPhonePlatform_BLUEI             WebcPhonePlatform_WebcPhonePlatform = 21
	WebcPhonePlatform_WebcPhonePlatform_FBLITEA           WebcPhonePlatform_WebcPhonePlatform = 22
	WebcPhonePlatform_WebcPhonePlatform_GREENA            WebcPhonePlatform_WebcPhonePlatform = 23
	WebcPhonePlatform_WebcPhonePlatform_GREENI            WebcPhonePlatform_WebcPhonePlatform = 24
	WebcPhonePlatform_WebcPhonePlatform_IGDA              WebcPhonePlatform_WebcPhonePlatform = 25
	WebcPhonePlatform_WebcPhonePlatform_IGDI              WebcPhonePlatform_WebcPhonePlatform = 26
	WebcPhonePlatform_WebcPhonePlatform_IGLITEA           WebcPhonePlatform_WebcPhonePlatform = 27
	WebcPhonePlatform_WebcPhonePlatform_MLITEA            WebcPhonePlatform_WebcPhonePlatform = 28
	WebcPhonePlatform_WebcPhonePlatform_MSGRA             WebcPhonePlatform_WebcPhonePlatform = 29
	WebcPhonePlatform_WebcPhonePlatform_MSGRI             WebcPhonePlatform_WebcPhonePlatform = 30
	WebcPhonePlatform_WebcPhonePlatform_MSGRP             WebcPhonePlatform_WebcPhonePlatform = 31
	WebcPhonePlatform_WebcPhonePlatform_MSGRW             WebcPhonePlatform_WebcPhonePlatform = 32
	WebcPhonePlatform_WebcPhonePlatform_IGDW              WebcPhonePlatform_WebcPhonePlatform = 33
	WebcPhonePlatform_WebcPhonePlatform_PAGE              WebcPhonePlatform_WebcPhonePlatform = 34
	WebcPhonePlatform_WebcPhonePlatform_MSGRDM            WebcPhonePlatform_WebcPhonePlatform = 35
	WebcPhonePlatform_WebcPhonePlatform_MSGRDW            WebcPhonePlatform_WebcPhonePlatform = 36
	WebcPhonePlatform_WebcPhonePlatform_MSGROM            WebcPhonePlatform_WebcPhonePlatform = 37
	WebcPhonePlatform_WebcPhonePlatform_MSGROC            WebcPhonePlatform_WebcPhonePlatform = 38
	WebcPhonePlatform_WebcPhonePlatform_MSGRM             WebcPhonePlatform_WebcPhonePlatform = 43
	WebcPhonePlatform_WebcPhonePlatform_IGDM              WebcPhonePlatform_WebcPhonePlatform = 44
	WebcPhonePlatform_WebcPhonePlatform_WEARM             WebcPhonePlatform_WebcPhonePlatform = 45
	WebcPhonePlatform_WebcPhonePlatform_CAPI              WebcPhonePlatform_WebcPhonePlatform = 46
	WebcPhonePlatform_WebcPhonePlatform_XR                WebcPhonePlatform_WebcPhonePlatform = 47
	WebcPhonePlatform_WebcPhonePlatform_MACOS             WebcPhonePlatform_WebcPhonePlatform = 48
	WebcPhonePlatform_WebcPhonePlatform_WAMETA_REPL       WebcPhonePlatform_WebcPhonePlatform = 49
	WebcPhonePlatform_WebcPhonePlatform_ARDEV             WebcPhonePlatform_WebcPhonePlatform = 50
	WebcPhonePlatform_WebcPhonePlatform_WEAROS            WebcPhonePlatform_WebcPhonePlatform = 51
	WebcPhonePlatform_WebcPhonePlatform_MSGRVR            WebcPhonePlatform_WebcPhonePlatform = 52
	WebcPhonePlatform_WebcPhonePlatform_BLUEW             WebcPhonePlatform_WebcPhonePlatform = 53
	WebcPhonePlatform_WebcPhonePlatform_IPHONEWAMETATEST  WebcPhonePlatform_WebcPhonePlatform = 54
	WebcPhonePlatform_WebcPhonePlatform_MSGRAR            WebcPhonePlatform_WebcPhonePlatform = 57
	WebcPhonePlatform_WebcPhonePlatform_IPAD              WebcPhonePlatform_WebcPhonePlatform = 58
	WebcPhonePlatform_WebcPhonePlatform_WAVOIP_CLI        WebcPhonePlatform_WebcPhonePlatform = 59
	WebcPhonePlatform_WebcPhonePlatform_MSGRT             WebcPhonePlatform_WebcPhonePlatform = 60
	WebcPhonePlatform_WebcPhonePlatform_IGDT              WebcPhonePlatform_WebcPhonePlatform = 61
	WebcPhonePlatform_WebcPhonePlatform_ANDROIDWAMETATEST WebcPhonePlatform_WebcPhonePlatform = 62
	WebcPhonePlatform_WebcPhonePlatform_MSGRSG            WebcPhonePlatform_WebcPhonePlatform = 63
	WebcPhonePlatform_WebcPhonePlatform_IGDSG             WebcPhonePlatform_WebcPhonePlatform = 64
	WebcPhonePlatform_WebcPhonePlatform_INTEROP           WebcPhonePlatform_WebcPhonePlatform = 65
	WebcPhonePlatform_WebcPhonePlatform_INTEROP_MSGR      WebcPhonePlatform_WebcPhonePlatform = 66
	WebcPhonePlatform_WebcPhonePlatform_IGDVR             WebcPhonePlatform_WebcPhonePlatform = 67
	WebcPhonePlatform_WebcPhonePlatform_WASG              WebcPhonePlatform_WebcPhonePlatform = 68
	WebcPhonePlatform_WebcPhonePlatform_BLUEVR            WebcPhonePlatform_WebcPhonePlatform = 69
	WebcPhonePlatform_WebcPhonePlatform_TEST              WebcPhonePlatform_WebcPhonePlatform = 9
	WebcPhonePlatform_WebcPhonePlatform_UNKNOWN           WebcPhonePlatform_WebcPhonePlatform = 10
)

type WebcWebPlatform_WebcWebPlatform int32

const (
	WebcWebPlatform_WebcWebPlatform_WEB            WebcWebPlatform_WebcWebPlatform = 1
	WebcWebPlatform_WebcWebPlatform_WIN32          WebcWebPlatform_WebcWebPlatform = 2
	WebcWebPlatform_WebcWebPlatform_DARWIN         WebcWebPlatform_WebcWebPlatform = 3
	WebcWebPlatform_WebcWebPlatform_IOS_TABLET     WebcWebPlatform_WebcWebPlatform = 4
	WebcWebPlatform_WebcWebPlatform_ANDROID_TABLET WebcWebPlatform_WebcWebPlatform = 5
	WebcWebPlatform_WebcWebPlatform_WINSTORE       WebcWebPlatform_WebcWebPlatform = 6
	WebcWebPlatform_WebcWebPlatform_MACSTORE       WebcWebPlatform_WebcWebPlatform = 7
	WebcWebPlatform_WebcWebPlatform_DARWIN_BETA    WebcWebPlatform_WebcWebPlatform = 8
	WebcWebPlatform_WebcWebPlatform_WIN32_BETA     WebcWebPlatform_WebcWebPlatform = 9
	WebcWebPlatform_WebcWebPlatform_PWA            WebcWebPlatform_WebcWebPlatform = 10
	WebcWebPlatform_WebcWebPlatform_WIN_HYBRID     WebcWebPlatform_WebcWebPlatform = 11
)
