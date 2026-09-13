package dev.devsquad.task.domain;

/** 11-시스템-아키텍처 §5.1 Task 상태. */
public enum TaskStatus {
    QUEUED, RUNNING, WAITING_APPROVAL, PAUSED, BLOCKED, COMPLETED, FAILED, CANCELLED;

    public boolean isTerminal() {
        return this == COMPLETED || this == FAILED || this == CANCELLED;
    }
}
