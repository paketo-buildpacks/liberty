### How to apply an iFix to the Liberty runtime

An iFix can be applied to the liberty runtime using the `external configuration package` feature.

Requirements to install ifixes:
 * Only the archive versions of Liberty ifixes are supported
 * The ifixes are zipped up in a directory named `ifixes`
 ```console
 zip ifixes.zip ifixes\*
 ```

 For example, the contents of the `ifixes` directory should look like the following:
 ```console
 ifixes/
 ifixes/220002-wlp-archive-ifph12345.jar
 ifixes/220002-wlp-archive-ifph67890.jar
 ```


Obtain the sha256 of the zip containing the ifixes:
 ```console
$ shasum -a 256 ifixes.zip
 dd464bd1e278123c00ce7b1fb21dd63d0441b3cf9877d0a1b2284ad01abd061a ifixes.zip
$
```

Set the `BP_LIBERTY_EXT_CONF_URI`, `BP_LIBERTY_EXT_CONF_SHA256` and `BP_LIBERTY_EXT_CONF_VERSION` environment variables on the `pack build` command and specify the --volume parameter:
```console
pack build myapp --env BP_JAVA_APP_SERVER=liberty --env BP_LIBERTY_EXT_CONF_URI=file:///tmp/ifixes/ifixes.zip --env BP_LIBERTY_EXT_CONF_SHA256=dd464bd1e278123c00ce7b1fb21dd63d0441b3cf9877d0a1b2284ad01abd061a --env BP_LIBERTY_EXT_CONF_VERSION=1.0.0 --volume /path/to/ifixes.zip:/tmp/ifixes/ifixes.zip
```
The build output will show the iFix being applied:
```console
[builder]   Open Liberty Config: Contributing to layer
[builder]     Copying /cnb/buildpacks/paketo-buildpacks_liberty/{{.version}}/templates/app.tmpl
[builder]     Copying /cnb/buildpacks/paketo-buildpacks_liberty/{{.version}}/templates/features.tmpl
[builder]   Open Liberty External Configuration 1.0.0
[builder]     Downloading from file:///tmp/ifixes/ifixes.zip
[builder]     Verifying checksum
[builder]     Expanding to /layers/paketo-buildpacks_liberty/base/conf
[builder] Installing 210012-wlp-archive-ifph12345.jar
[builder]       Applying fix to Liberty install directory at /layers/paketo-buildpacks_liberty/open-liberty-runtime-full now.
[builder]       	lib/com.ibm.ws.security.wim.adapter.ldap_1.0.59.cl211220220114-0527.jar
[builder]       Fix has been applied successfully.
[builder]       Successfully extracted all product files.
```
