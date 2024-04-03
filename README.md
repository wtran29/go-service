# Ultimate Go: Web Services with Kubernetes 4.1

This project was based on the course taught by Bill Kennedy where different topics were discussed such as implementing a web service through domain driven development and bringing it to life. 
The main takeaways were his views on design philosophy.

```
1. We don't make things easy to do, we make things easy to understand.
2. Precision - it is better to create literal structs than importing types into the program.
Most of the time when creating APIs, they are private. When we create literal structs, this offers a level of precision.
If we bring in a type, we bring everything rather than the direct object of what we're looking for.
```

## Design Philosophy

```
1. Clarity and Readability - Make code more readable by providing context. When someone else reads the code,
they can understand what the data structure represents.
2. Simplicity - Write straight forward code, easy to understand and avoids unncessary complexity.
3. Pragmatism - Focus on practical solutions that meet the specific requirements and constraints of the project
rather than adhering to theoretical principles.
4. Minimize Dependencies - Being mindful of libraries and frameworks to include to the project. Opt for simpler and
lightweight solutions.
5. Testing and Maintainability - Write testable code and design systems that stand the test of time. 
```

## Project Layers

Aim to keep the project to 5 layers or less and packages should not be importing/exporting from each other. The purpose is to mimimize technical debt.

```
1. Application - this is the presentational layer where project starts up, shuts down, receives internal input
and returns external output.
2. Business - this is where the business logic is stored, including core business packages, CRUD layer of database,
system oriented package that is tied to business problem, package specific to web app
3. Foundation - this is a layer that is not tied to any business problem and is reusable, no logging
4. Vendor - package dependencies that are brought in from `go mod vendor`
5. Containers - layer for config files related to Docker, Kubernetes, etc
```
