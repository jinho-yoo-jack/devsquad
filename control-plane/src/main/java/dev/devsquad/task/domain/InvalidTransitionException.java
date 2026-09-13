package dev.devsquad.task.domain;

/** 409 INVALID_TRANSITION (15-API §7). */
public class InvalidTransitionException extends RuntimeException {
    private final String from;
    private final String event;

    public InvalidTransitionException(Enum<?> from, String event) {
        super("invalid transition: " + from + " --" + event + "--> ?");
        this.from = from.name();
        this.event = event;
    }

    public String from() { return from; }
    public String event() { return event; }
}
