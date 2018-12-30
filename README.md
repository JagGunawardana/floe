Floe 
====
[![Build Status](https://travis-ci.org/floeit/floe.svg?branch=master)](https://travis-ci.org/floeit/floe)

A workflow engine, well suited to long running business process execution, for example:

* Continuous delivery.
* Continuous integration.
* Customer onboarding.

Demo
----

There is a demo server that checks out and builds the floe code (and the floe docs)...

[DEMO](https://demo.floe.it/app/flows/floe) (enter any username and password - it is not checked)

User Docs
---------
[Documentation for using floe.](http://www.floe.it/) - including quick start.

Building
--------
1. Clone the repo
2. `go install ./...`
3. `go test ./...` (optional `-tags=integration_test`)

 
Floe Terminology 
----------------
Flows are coordinated by nodes issuing events to which other nodes `listen`.

`Host`    - Any named running `floe` service, it could be many such processes on single compute unit (vm, container), or one each.

`Flow`    - A description of a set of `nodes` linked by events. A specific instance of an flow is a `Run`

`Node`    - A config item that can respond to and issue events. A Node is part of a flow. Certain Nodes can Execute actions.

The types of Node are. 

* `Triggers` - These start a run for their flow, and are http or polling nodes that respond to http requests or changes in a polled entity.
* `Tasks` - Nodes that on responding to an event execute some action, and then emit an event.
* `Merges` - A node that waits for `all` or `one` of a number of events before issuing its event.

`Run`      - A specific invocation of a flow, can be in one of three states Pending, Active, Archive.

`Workspace`- A place on disc for this run where most of the run actions should take place. Since flow can execute arbitrary scripts, there is no guarantee that mutations to storage are constrained to this workspace. It is up to the script author to isolate (e.g. using containers, or great care!) 

`RunRef`   - An 'adopted' RunRef is a globally unique compound reference that resolves to a specific Run.

`Hub`      - Is the central routing object. It instantiates Runs and executes actions on Nodes in the Run based on its config from any events it observes on its queue.

`Event`    - Events are issued after a node has completed its duties. Other nodes are configured to listen for events. Certain other events are emitted that announce other state changes. Events are propagated to any clients connected via web sockets.

`Queue`    - The hub event queue is the central chanel for all events.

`RunStore` - the hub references a run store that can persist the following lists - representing the three states of a run..
* `Pending` - Runs waiting to be executed, a Run on this list is called a Pend.
* `Active` - Runs that are executing, and will be matched to events with matching adopted RunRefs. 
* `Archive` - Runs that are have finished executing.

Life cycle of a flow
--------------------
When a trigger event arrives on the queue that matches a flow, the event reference will be considered 'un-adopted' this means it has not got a full run reference. A pending run is created with a globally unique compound reference (now adopted) - this reference (and some other meta data) is added to the pending list of the host that adopted it as a 'Pend' - this may not be the host that executes the run later. (These were called TODO's but that has a very particular meaning!)

A background process tries to assign Pend's to any host where the `HostTags` match, and where there are no Runs already matching the `ResourceTags` asked for - this allows certain nodes to be assigned to certain Runs, and to serialise Runs that need exclusive access to any third party, or other shared resources.

Once a Pend has been dispatched for execution it is moved out of the adopting Pending list and into the Active List on the executing host.

When one of the end conditions for a Run is met the Run is moved out of the Active list and into the Archive list on the host that executed the Run.

All of this is dealt with in the `Hub` the files are divided into three:

* `hub_setup.go` - The Hub definition and initial setup code.
* `hub_pend.go` - Code that handle events that trigger a pending run, and dispatches them to available hosts.
* `hub_exec.go` - Code that accepts a pending run and activates it, directs events to task nodes, and Executes tasks.

Config
------
It is best to read the [Config file Documentation.](http://www.floe.it/#config)

Development
-----------
The web assets are shipped in the binary as 'bindata' so if you change the web stuff then run `go generate ./server` to regenerate the `bindata.go`

To run a `Host` ...

`floe -tags=linux,go,couch -admin=123456 -host_name=h1 -pub_bind=127.0.0.1:8080`

During dev you can use the `webapp` folder directly by passing in `-dev=true` to the floe command.


TLS Testing
-----------
Generate a self signed cert and key and add them on the command line

```
openssl req \
    -x509 \
    -nodes \
    -newkey rsa:2048 \
    -keyout server.key \
    -out server.crt \
    -days 3650 \
    -subj "/C=GB/ST=London/L=London/O=Global Security/OU=IT Department/CN=*"
```

Working on a new Flow
---------------------
Working on a new flow outside of the development environment, for example if you have just downloaded `floe` or even if you have built in your go environment and want to test a flow isolated from the dev env.

### 1. From a local folder.
Launch the floe you have downloaded or built but point it at a config and root folder somewhere else. 

`floe -tags=linux,go,couch -admin=123456 -host_name=h1 -conf=/somepath/testfloe/config.yml -pub_bind=127.0.0.1:8080`

Deploying to AWS
----------------
There is an image available that can bootstrap a floe instance `ami-006defacf6ec36202`. This image contains the Letsencrypt certbot, supervisord, git and floe.

Floe can bind its web handlers to the public and private ip's and run TLS on each independently. For instance if you are not terminating your inbound requests on a TLS enabled balancer or reverse proxy then you can bind floe to the external IP and serve TLS on that, whilst serving plain http for the floe to floe cluster.

Running floe directly on the vm means you dont benefit from the fully hermetic approach of using an ephemeral container, but floe can be used to create hermetic builds with some care and set up; the flow itself can download and install tooling into the workspace and use only these tools, of course this has an overhead, and you may want to make the tools you need to download available in S3, however many tools are well cached by amazon (Golang for example).

### Environment - WARNING
Whist floe attempts to only set env vars within the scope of its sub processes, there is nothing in particular to stop you writing scripts or programs that alter the global environment. Similarly all file activity is generally expected to be within the run workspace, but you could alter global shared storage in your flow. 

Given all that - you may still be happy that you ave built a well controlled image that already has the tools at known versions, and you are happy that your builds are repeatable and that no action of previous builds are mutating any installed components or otherwise altering the environment, and are therefore effectively safe enough.

There is an example `start.sh` script with typical command line options. You can use this as an example of how to launch `floe` in your own vm.




