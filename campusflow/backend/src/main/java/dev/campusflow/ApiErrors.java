package dev.campusflow;

import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.http.*;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.*;

@RestControllerAdvice
public class ApiErrors {

  @ExceptionHandler(DomainException.class)
  ResponseEntity<ProblemDetail> domain(DomainException ex) {
    return ResponseEntity.status(ex.status).body(
      ProblemDetail.forStatusAndDetail(
        HttpStatusCode.valueOf(ex.status),
        ex.getMessage()
      )
    );
  }

  @ExceptionHandler(MethodArgumentNotValidException.class)
  ResponseEntity<ProblemDetail> validation(MethodArgumentNotValidException ex) {
    String detail = ex
      .getBindingResult()
      .getFieldErrors()
      .stream()
      .map(e -> e.getField() + ": " + e.getDefaultMessage())
      .findFirst()
      .orElse("Invalid input");
    return ResponseEntity.badRequest().body(
      ProblemDetail.forStatusAndDetail(HttpStatus.BAD_REQUEST, detail)
    );
  }

  @ExceptionHandler(DataIntegrityViolationException.class)
  ResponseEntity<ProblemDetail> conflict() {
    return ResponseEntity.status(409).body(
      ProblemDetail.forStatusAndDetail(
        HttpStatus.CONFLICT,
        "Request conflicts with existing data"
      )
    );
  }
}
