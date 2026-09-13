package dev.devsquad.approval.domain;

/** 400 FEEDBACK_REQUIRED. */
public class FeedbackRequiredException extends RuntimeException {
    public FeedbackRequiredException() { super("reject 에는 feedback 이 필요합니다"); }
}
