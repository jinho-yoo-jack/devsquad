package dev.devsquad.approval.domain;

import java.time.Instant;

/** 409 APPROVAL_ALREADY_DECIDED — 다른 채널(Discord/웹)에서 먼저 결정됨. */
public class AlreadyDecidedException extends RuntimeException {
    private final String decidedVia;
    private final Instant decidedAt;

    public AlreadyDecidedException(String decidedVia, Instant decidedAt) {
        super("approval already decided via " + decidedVia + " at " + decidedAt);
        this.decidedVia = decidedVia;
        this.decidedAt = decidedAt;
    }

    public String decidedVia() { return decidedVia; }
    public Instant decidedAt() { return decidedAt; }
}
