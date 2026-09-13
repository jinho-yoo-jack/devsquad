package dev.devsquad.common;

import dev.devsquad.approval.domain.AlreadyDecidedException;
import dev.devsquad.approval.domain.FeedbackRequiredException;
import dev.devsquad.task.domain.InvalidTransitionException;
import java.util.Map;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.orm.ObjectOptimisticLockingFailureException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

/** 도메인 예외 → 15-API §7 오류 코드. */
@RestControllerAdvice
public class GlobalExceptionHandler {

    @ExceptionHandler(AlreadyDecidedException.class)
    ResponseEntity<ApiError> alreadyDecided(AlreadyDecidedException e) {
        return ResponseEntity.status(HttpStatus.CONFLICT).body(new ApiError("APPROVAL_ALREADY_DECIDED", e.getMessage(),
            Map.of("decided_via", String.valueOf(e.decidedVia()), "decided_at", String.valueOf(e.decidedAt()))));
    }

    /** 동시 결정에서 낙관적 락에 진 쪽 — 사용자에게는 "이미 처리됨" 으로 보인다. */
    @ExceptionHandler(ObjectOptimisticLockingFailureException.class)
    ResponseEntity<ApiError> optimisticLock(ObjectOptimisticLockingFailureException e) {
        return ResponseEntity.status(HttpStatus.CONFLICT)
            .body(ApiError.of("APPROVAL_ALREADY_DECIDED", "다른 채널에서 먼저 처리되었습니다"));
    }

    @ExceptionHandler(InvalidTransitionException.class)
    ResponseEntity<ApiError> invalidTransition(InvalidTransitionException e) {
        return ResponseEntity.status(HttpStatus.CONFLICT).body(new ApiError("INVALID_TRANSITION", e.getMessage(),
            Map.of("from", e.from(), "event", e.event())));
    }

    @ExceptionHandler(FeedbackRequiredException.class)
    ResponseEntity<ApiError> feedbackRequired(FeedbackRequiredException e) {
        return ResponseEntity.badRequest().body(ApiError.of("FEEDBACK_REQUIRED", e.getMessage()));
    }

    @ExceptionHandler(IllegalArgumentException.class)
    ResponseEntity<ApiError> badRequest(IllegalArgumentException e) {
        return ResponseEntity.badRequest().body(ApiError.of("BAD_REQUEST", e.getMessage()));
    }
}
