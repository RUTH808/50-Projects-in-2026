package dev.campusflow;

import static org.junit.jupiter.api.Assertions.*;

import java.time.Instant;
import org.junit.jupiter.api.Test;

class ReservationValidationTest {

  @Test
  void pastAndReversedIntervalsFailBeforeDatabaseAccess() {
    var service = new CampusService(null);
    Instant now = Instant.now();
    assertEquals(
      400,
      assertThrows(DomainException.class, () ->
        service.reserve(1, 1, now.minusSeconds(60), now.plusSeconds(60), "Test")
      ).status
    );
    assertEquals(
      400,
      assertThrows(DomainException.class, () ->
        service.reserve(1, 1, now.plusSeconds(120), now.plusSeconds(60), "Test")
      ).status
    );
    assertEquals(
      400,
      assertThrows(DomainException.class, () ->
        service.reserve(
          1,
          1,
          now.plusSeconds(60),
          now.plusSeconds(8 * 86400),
          "Test"
        )
      ).status
    );
  }
}
