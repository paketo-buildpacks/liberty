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

Set specify the --volume parameter mapping your local `ifixes/` directory to `/ifixes` in the container:

```console
pack build myapp --volume /path/to/ifixes:/ifixes
```

The build output will show the iFix being applied:

```console
  Open Liberty (All Features) 22.0.3: Contributing to layer
    Downloading from file:///bindings/dependency-mapping/binaries/openliberty-runtime-22.0.0.3.zip
    Verifying checksum
    Expanding to /layers/paketo-buildpacks_liberty/open-liberty-runtime-full
    Installing iFix test-ifix.jar
      Installing 210012-wlp-archive-ifph12345.jar
      Applying fix to Liberty install directory at /layers/paketo-buildpacks_liberty/open-liberty-runtime-full now.
        lib/com.ibm.ws.security.wim.adapter.ldap_1.0.59.cl211220220114-0527.jar
      Fix has been applied successfully.
      Successfully extracted all product files.
```
