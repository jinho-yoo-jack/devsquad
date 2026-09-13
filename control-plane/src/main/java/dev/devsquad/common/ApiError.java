package dev.devsquad.common;

import java.util.Map;

/** 15-API 명세 공통 오류 응답. */
public record ApiError(String code, String message, Map<String, Object> details) {
    public static ApiError of(String code, String message) { return new ApiError(code, message, Map.of()); }
}
