# Explain K8s Generator

Generate a JSON array of `kubectl explain` details for specific kubernetes resources.

### Okay, what now?

Alright, so, kubernetes comes with a CLI tool called `kubectl`. This CLI can, among (many) other things, give official and detailed "explanations" of each k8s resource, as well as any field or subfield inside of that resource. What this leads to, is a recursive structure of detailed explanations that one can explore using the CLI. For example, lets get an explanation for the `pod` resource:

`kubectl explain pods`
```
DESCRIPTION:
     Pod is a collection of containers that can run on a host. This resource is
     created by clients and scheduled onto hosts.

FIELDS:
    ...
    
    metadata     <Object>
        Standard object's metadata. More info:
        https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
    
    ...
```
(shortened for brevity)

And we can get an explanation of what fields are in the `metadata` object of `pods` with:

`kubectl explain pods.metadata`

```
DESCRIPTION:
     Standard object's metadata. More info:
     https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata

     ObjectMeta is metadata that all persisted resources must have, which
     includes all objects users must create.

FIELDS:
    ...
```
(shortened for brevity)

And if `pod.metadata` had any fields of type `Object`, we could `kubectl explain` those as well. In fact, we can get a high level overview of everything in the `pod` resource with:

`kubectl explain pods --recursive`

```
DESCRIPTION:
     Pod is a collection of containers that can run on a host. This resource is
     created by clients and scheduled onto hosts.

FIELDS:
   apiVersion   <string>
   kind <string>
   metadata     <Object>
      annotations       <map[string]string>
      clusterName       <string>
      creationTimestamp <string>
      deletionGracePeriodSeconds        <integer>
      deletionTimestamp <string>
      finalizers        <[]string>
      generateName      <string>
      generation        <integer>
      initializers      <Object>
         pending        <[]Object>
            name        <string>
            ...
```
(shortened for brevity)

So, if you notice, every resource can be explained. Every resource contains a set of fields that can be explained. Every field that is of type `Object` (or `[]Object`) contains sub-fields that can then also be explained. This is the recursive structure that emerges from k8s resources.

Now, what if someone was crazy enough to write an application that would programmatically step through a list of resources, get the explanation of each one (and all of its sub-field's explanations), parse through the *whitespace* in the resulting explanations to determine how deep the recursion goes, then erect a custom recursive structure through the use of go structs with the intent of encoding it into JSON for further consumption downstream in, say, a [UI](https://www.github.com/Insulince/explain-k8s-ui) of some sort. That would be crazy right??? Well, it certainly isn't sane. But it's done, so sue me.

## Explanation Model

Each explanation is captured and stored in an instance of the following model:

```go
type Explanation struct {
	Name        string        `json:"name"`
	FullName    string        `json:"fullName"`
	Type        string        `json:"type"`
	Description string        `json:"description"`
	Fields      []Explanation `json:"fields"`
}
```

`Name` - Stores the name of the current resource or field. So `pod.metadata` would be `metadata`

`FullName` - Stores full name of the current resource or field. This would be a reflection of how deep into the k8s explanation structure this instance is. An example would be `pod.metadata.creationTimestamp`.

`Type` - The type provided k8s *without* the `<` and `>` surrounding it. So `<string>` would become just `string`.

`Description` - This is a raw capture of the `DESCRIPTION` section of a `kubectl explain` on a resource or field. Additional whitespace and newlines are stripped. This may lead to a few spots where spaces feel awkward, but it should be close enough in most cases.

`Fields` - This contains all fields of the current resource, or sub fields of the current field. What you will find is that only explanations of items that are either resources (top-level structures) or of type `Object` (or `[]Object`) will ever have this field populated with anything more than an empty slice. All it will contain is another instance of this object, but for the subfield. They will be sorted by `Name` (not `FullName`) by the end of execution.

## Usage

NOTE: *This process was created on a v1.9.5 client against a v1.10.11 server. Results may vary or not come back at all if your versions differ.*

To start, is up to you to provide a list of resource names in an input file. The default location for this file is `./in/resourceNames.txt` (can be changed with environment variables). This file should contain *only* valid k8s resource names separated by newlines.

Once that is set up, you are ready to run the application which will perform a **ton** of `kubectl explain` requests against your **current kubectl context**. Please be sure you are configured to not damage your server with this many requests. I set up a local k8s cluster and context to run these commands against so I wouldn't have to worry about harming anything.

Once the process is complete, the generated `Explanation` structures will be marshalled into JSON and saved into an output file. The default location for this file is `./out/output.json` (can also be changed with environment variables, but will always generate *json*).

## Environment Variables

`VERBOSE_MODE` (boolean) - Run in verbose mode. This just means displaying things while running like which resource is being processed, as well as what the current sub-field is, as well as displaying timestamps and other interesting information. Default is `true`

`RESOURCE_NAMES_FILE_LOCATION` (string) - Where the list of resource names are located. The detault is `./in/resourceNames.txt`.

`OUTPUT_FILE_LOCATION` (string) - Where the generated JSON should be put. Default is `./out/output.json`

`PARALLEL_MODE` (boolean) - Run with `runtime.GOMAXPROCS(runtime.NumCpu())` or not. The effect does not seem to be major, but included anyway. Default is `true`.
