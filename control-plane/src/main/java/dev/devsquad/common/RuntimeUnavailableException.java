package dev.devsquad.common;

/** 503 RUNTIME_UNAVAILABLE — Agent Runtime 에 닿지 않음. Task 는 queued 로 유지되고 RunRecoveryJob 이 재시도. */
public class RuntimeUnavailableException extends RuntimeException {
    public RuntimeUnavailableException(Throwable cause) { super("agent runtime unavailable: " + cause.getMessage(), cause); }
}
