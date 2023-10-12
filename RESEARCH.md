# Research
A links for various parts of the SDS framework.

# Videos

* [Chris Hines: A Netflix architecture in golang](https://www.youtube.com/watch?v=h0s8CWpIKdg)

# Books

## Let's go further
Advanced patterns for building API and web applications.

Website: [alexedwards.net](https://lets-go-further.alexedwards.net/)

> **Todo**
> 
> Apply the techniques to improve the SDS code.

> **Todo**
> 
> Make sure to make easy to create apps with SDS applying the practices from the code.

> **Todo**
> 
> Answer to the question, can we use the advanced patterns in the SDS to add automatically?
> If the answer is yes, design the SDS architecture to include the patterns internally.
> If the answer is no, then make sure that they are createable using services.

## Architecture of Open Source Applications
Website: [Aosabook.org](https://aosabook.org/en/)

A four-series research books from the open source project creators.

> **Todo**
> 
> Utilize the things to make SDS Framework easier what kind of apps and
> What kind of architecture they will have.

> **Todo**
> 
> Try to run each example with SDS Framework, it must be faster.

The volume 3, *The Performance of Open Source Applications* is needed
during the SDS optimization for production ready use.

## Software design by Example
The best way to learn design is by example.
And some of the best examples come from the tools that programmers use.
We will learn how to create small versions of the tools,
regexp, testing framework, browser layout engines, back up systems.

Website 1: [Software design by Javascript](https://third-bit.com/sdxjs/)

Website 2: [Software design by Python](https://third-bit.com/sdxpy/)

> **Todo**
> 
> Read, to apply the techniques for the optimization of the SDS Framework.

## Build orchestration tools in go from scratch
Website: [Manning.com](https://livebook.manning.com/book/build-an-orchestrator-in-go/brief-table-of-contents/welcome/)

This book is intended to have deep understanding of how kubernetes and other orchestration books work.

> **Todo**
> 
> Read it to optimize the context interface. As well as to create a kubernetes, and other orchestration tool contexts.

---
# Context

### Articles

### Nginx Blog about Microservices
A seven part series articles
1. [Introduction to Microservices](https://www.nginx.com/blog/introduction-to-microservices/)
2. [Using API Gateway](https://www.nginx.com/blog/building-microservices-using-an-api-gateway/)
3. [Inter-Process communication](https://www.nginx.com/blog/building-microservices-inter-process-communication/)
4. [Service Discovery](https://www.nginx.com/blog/service-discovery-in-a-microservices-architecture/)
5. [Event driven data management](https://www.nginx.com/blog/event-driven-data-management-microservices/)
6. [Deployment Strategy](https://www.nginx.com/blog/deploying-microservices/)
7. [Refactoring Monolith](https://www.nginx.com/blog/refactoring-a-monolith-into-microservices/)

---

## Apache Mesos context
*Apache Mesos* abstracts CPU, memory, storage, and other compute resources away from machines (physical or virtual), 
enabling fault-tolerant and elastic distributed systems to easily be 
built and run effectively.

Website: [mesos.apache.org](https://mesos.apache.org/)

**It's doing the same thing as the context**.

> **Todo**
> 
> Research it by running sample example

> **Todo**
> 
> Answer the question, can it help us to save the development time

> **Todo**
> 
> Write a documentation how to use it as a *Mesos* context.

### Marathon
Deploy and manage containers including docker on top of Apache Mesos.

Website: [marathon.github.io](https://mesosphere.github.io/marathon/)
Github: [mesossphere/marathon](https://github.com/mesosphere/marathon)
*4.1k stars as of October 2023*.

**If the mesos context is created, perhaps create it with marathon?**.

> **Todo**
> 
> Research it by running sample application

> **Todo**
>
> Answer to the following questions:
> **Can it help us to build the Mesos**
> If the answer is yes, then plan the architecture.

> **Todo**
> 
> Especially research by running sample app the service discovery and load balancing
> 
> [doc/service-discovery](https://mesosphere.github.io/marathon/docs/service-discovery-load-balancing.html)

---

# Docker context

#### Registrator
This is a docker service that registers and deregisters other containers.

Github: [gliderlabs/registrator](https://github.com/gliderlabs/registrator/)
* 4.6k stars as of October 2023*.

**I don't know how it works.**.
**From what I see on the internet, it works on par with other orchestration tools**.

> **Todo**
> 
> Run it, to see how it works.

> **Todo**
> 
> If we can use it, then write a plan how to use it in *docker* context.

---

# Debugging dashboard
A dashboard that shows the microservice parameters.

Basically, the SDS framework must build the following dashboards automatically:
[Reddit post](https://www.reddit.com/r/softwarearchitecture/comments/170fc6e/how_to_generate_infographics_like_this/).

Plus, animated. Plus exportable.
The users may use it during the prototyping.
Write the sample code with the mocked data. Then export the diagrams to your investors.

> **Todo**
> 
> Maybe create an SDS hub for dashboards.
> So users can share it with others. The dashboard will be **interactive**.
> The dashboard has a link to the code.
> Then dashboard can run the tests or allow users to create tests with GUI.
> 
> **Plan it**.
> 
> The Paid version includes support for the private repo.

## Books

### Javascript for Data Science
Website: [third-bit.com/j4ds](https://third-bit.com/js4ds/)

It teaches Javascript and React framework to build a diagram rich websites.

**It could be used for an animated request flow of the metrics**.

**Two variants to use**
**1. Show the whole app and the service relationship.**
**In this case, Mouse hover on the Service shows.**
**machine parameters, handlers, configuration and api routes**.


---

# Testing

* [`go test` docs](https://pkg.go.dev/cmd/go/internal/test)
* [Youtube: David Chaney. Three go profiling techniques](https://www.youtube.com/watch?v=nok0aYiGiYA)

## Coverage
For testing the SDS framework itself.
[testing/coverage](https://go.dev/testing/coverage/)

> **Todo**
> 
> Also, when we design the AI assistant, make sure it also uses the coverage
> To show the developer that the app isn't coveraged yet.
> Coverage is needed for creating a fake data to use for finding the flow problems.

## Tracing and loggin

* [google cloud blog: distributed tracing in microservices](https://cloud.google.com/architecture/microservices-architecture-distributed-tracing/).