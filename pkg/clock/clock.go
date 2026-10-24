/*
Package clock provides the time the TAF assigns to its events, such as the updates of trust model instances, in
nanoseconds:
  - with the simulation build tag (simulation.go), the simulated time, advanced by the times of incoming messages
    (reference times of V2X CAMs, generation times of MBD reports, timestamps of TCH notifications); 0 before the
    first such message,
  - without it (wallclock.go), the wall-clock time as Unix time.
*/
package clock
