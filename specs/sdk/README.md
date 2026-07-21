# sdk/

One canonical interface, seven idiomatic bindings — not seven independent designs. The requirement that every SDK "expose exactly the same abstractions regardless of provider" is exactly why: seven files each inventing their own method names or error models would drift from each other over time, defeating the requirement it was supposed to satisfy. Each file here states the same nine operations in that language's own async idiom, error-handling convention, and naming case — diff any file against `typescript.md`'s canonical block to check for drift, that's what it's there for.
