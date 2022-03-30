### How to apply an iFix to the Liberty runtime

An iFix can be applied to the liberty runtime using a volume mount.

Requirements to install ifixes:

 * Only the archive versions of Liberty ifixes are supported
 * The ifixes are in a directory named `ifixes`

 For example, the contents of the `ifixes` directory should look like the following:
 ```console
 ifixes/
 ifixes/220002-wlp-archive-ifph12345.jar
 ifixes/220002-wlp-archive-ifph67890.jar
 ```

Specify the --volume parameter mapping your local `ifixes/` directory to `/ifixes` in the container:

```console
pack build myapp --env BP_JAVA_APP_SERVER=liberty --volume /path/to/ifixes:/ifixes
```

The build output will show the iFix being applied:

```console
[builder]   Open Liberty (All Features) 22.0.3: Contributing to layer
[builder]     Downloading from https://repo1.maven.org/maven2/io/openliberty/openliberty-runtime/22.0.0.3/openliberty-runtime-22.0.0.3.zip
[builder]     Verifying checksum
[builder]     Expanding to /layers/paketo-buildpacks_liberty/open-liberty-runtime-full
[builder]     Installing iFix 22003-wlp-archive-ifph44666.jar
[builder]       Applying fix to Liberty install directory at /layers/paketo-buildpacks_liberty/open-liberty-runtime-full now.
[builder]       	lib/com.ibm.ws.openapi.ui.private_1.0.62.cl220320220308-1516.jar
[builder]       	lib/com.ibm.ws.openapi.ui_1.0.62.cl220320220308-1516.jar
[builder]       	lib/com.ibm.ws.microprofile.openapi.ui_1.0.62.cl220320220308-1516.jar
[builder]       Fix has been applied successfully.
[builder]       Successfully extracted all product files.
```
