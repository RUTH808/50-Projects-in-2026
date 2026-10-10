package dev.campusflow;

import static org.junit.jupiter.api.Assertions.*;
import static org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.*;

import java.time.Instant;
import java.util.*;
import java.util.concurrent.*;
import org.junit.jupiter.api.*;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.web.servlet.AutoConfigureMockMvc;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.test.web.servlet.MockMvc;

/** Uses a real, disposable PostgreSQL database; TEST_DATABASE_URL is an explicit opt-in. */
@EnabledIfEnvironmentVariable(named = "TEST_DATABASE_URL", matches = ".+")
@SpringBootTest(
  properties = {
    "spring.datasource.url=${TEST_DATABASE_URL}",
    "spring.datasource.username=${TEST_DB_USER:campusflow}",
    "spring.datasource.password=${TEST_DB_PASSWORD:campusflow-local}",
  }
)
@AutoConfigureMockMvc
class CampusIntegrationTest {

  @Autowired
  CampusService service;

  @Autowired
  JdbcTemplate db;

  @Autowired
  MockMvc mvc;

  long equipmentId, studentId, secondId;
  Map<String, Object> student, staff;
  Instant start, end;

  @BeforeEach
  void setup() {
    db.execute(
      "TRUNCATE audit_event,maintenance_ticket,booking,equipment RESTART IDENTITY CASCADE"
    );
    student = service.actor("student@campusflow.local");
    staff = service.actor("staff@campusflow.local");
    studentId = ((Number) student.get("id")).longValue();
    secondId = db.queryForObject(
      "INSERT INTO app_user(name,email,password_hash,role) VALUES ('Second','second@campusflow.local','unused','STUDENT') ON CONFLICT(email) DO UPDATE SET name='Second' RETURNING id",
      Long.class
    );
    equipmentId = db.queryForObject(
      "INSERT INTO equipment(name,category,location) VALUES ('Camera','Media','Lab') RETURNING id",
      Long.class
    );
    start = Instant.now().plusSeconds(3600);
    end = start.plusSeconds(3600);
  }

  @Test
  void concurrentReservationsNeverDoubleBook() throws Exception {
    ExecutorService executor = Executors.newFixedThreadPool(12);
    CountDownLatch ready = new CountDownLatch(12),
      go = new CountDownLatch(1);
    try {
      List<Future<String>> requests = new ArrayList<>();
      for (int i = 0; i < 12; i++) {
        long user = db.queryForObject(
          "INSERT INTO app_user(name,email,password_hash,role) VALUES (?,?, 'unused','STUDENT') ON CONFLICT(email) DO UPDATE SET name=EXCLUDED.name RETURNING id",
          Long.class,
          "Concurrent " + i,
          "concurrent" + i + "@campusflow.local"
        );
        requests.add(
          executor.submit(() -> {
            ready.countDown();
            go.await();
            return (String) service
              .reserve(user, equipmentId, start, end, "Lab project")
              .get("status");
          })
        );
      }
      assertTrue(ready.await(5, TimeUnit.SECONDS));
      go.countDown();
      List<String> statuses = new ArrayList<>();
      for (var request : requests)
        statuses.add(request.get(20, TimeUnit.SECONDS));
      assertEquals(1, Collections.frequency(statuses, "PENDING"));
      assertEquals(11, Collections.frequency(statuses, "WAITLISTED"));
    } finally {
      executor.shutdownNow();
    }
  }

  @Test
  void cancellationPromotesOldestEligibleRequest() {
    var a = service.reserve(studentId, equipmentId, start, end, "A");
    var b = service.reserve(secondId, equipmentId, start, end, "B");
    service.transition(((Number) a.get("id")).longValue(), "cancel", student);
    assertEquals(
      "PENDING",
      db.queryForObject(
        "SELECT status FROM booking WHERE id=?",
        String.class,
        b.get("id")
      )
    );
    assertEquals(
      1L,
      db.queryForObject(
        "SELECT count(*) FROM audit_event WHERE action='WAITLIST_PROMOTED'",
        Long.class
      )
    );
  }

  @Test
  void maintenanceWaitsForEveryTicketToBeResolved() {
    var t1 = service.report(studentId, equipmentId, "Broken lens");
    var t2 = service.report(studentId, equipmentId, "Battery failure");
    var b = service.reserve(studentId, equipmentId, start, end, "Film");
    assertEquals("WAITLISTED", b.get("status"));
    service.resolve(
      ((Number) staff.get("id")).longValue(),
      ((Number) t1.get("id")).longValue()
    );
    assertEquals(
      "WAITLISTED",
      db.queryForObject(
        "SELECT status FROM booking WHERE id=?",
        String.class,
        b.get("id")
      )
    );
    service.resolve(
      ((Number) staff.get("id")).longValue(),
      ((Number) t2.get("id")).longValue()
    );
    assertEquals(
      "PENDING",
      db.queryForObject(
        "SELECT status FROM booking WHERE id=?",
        String.class,
        b.get("id")
      )
    );
  }

  @Test
  void ownershipAndTransitionsAreEnforced() {
    var b = service.reserve(secondId, equipmentId, start, end, "Project");
    long id = ((Number) b.get("id")).longValue();
    assertEquals(
      403,
      assertThrows(DomainException.class, () ->
        service.transition(id, "cancel", student)
      ).status
    );
    assertEquals(
      403,
      assertThrows(DomainException.class, () ->
        service.transition(id, "approve", student)
      ).status
    );
    service.transition(id, "approve", staff);
    assertEquals(
      409,
      assertThrows(DomainException.class, () ->
        service.transition(id, "approve", staff)
      ).status
    );
    assertEquals(
      409,
      assertThrows(DomainException.class, () ->
        service.transition(id, "checkout", staff)
      ).status
    );
  }

  @Test
  void databaseConstraintRejectsBypassingApplicationLock() {
    service.reserve(studentId, equipmentId, start, end, "First");
    assertThrows(
      org.springframework.dao.DataIntegrityViolationException.class,
      () ->
        db.update(
          "INSERT INTO booking(equipment_id,user_id,starts_at,ends_at,purpose,status) VALUES (?,?,?,?,'Bypass','CONFIRMED')",
          equipmentId,
          secondId,
          java.sql.Timestamp.from(start),
          java.sql.Timestamp.from(end)
        )
    );
  }

  @Test
  void securityAndCsrfAreEnforced() throws Exception {
    mvc.perform(get("/api/equipment")).andExpect(status().isUnauthorized());
    mvc
      .perform(
        get("/api/staff/audit").with(
          user("student@campusflow.local").roles("STUDENT")
        )
      )
      .andExpect(status().isForbidden());
    mvc
      .perform(
        post("/api/bookings")
          .with(user("student@campusflow.local").roles("STUDENT"))
          .contentType("application/json")
          .content("{}")
      )
      .andExpect(status().isForbidden());
    mvc
      .perform(
        post("/api/bookings")
          .with(user("student@campusflow.local").roles("STUDENT"))
          .with(csrf())
          .contentType("application/json")
          .content("{}")
      )
      .andExpect(status().isBadRequest());
  }

  @Test
  void adjacentIntervalsAreAllowedAndDuplicateRequestsRejected() {
    assertEquals(
      "PENDING",
      service.reserve(studentId, equipmentId, start, end, "First").get("status")
    );
    assertEquals(
      "PENDING",
      service
        .reserve(secondId, equipmentId, end, end.plusSeconds(3600), "Adjacent")
        .get("status")
    );
    assertEquals(
      409,
      assertThrows(DomainException.class, () ->
        service.reserve(studentId, equipmentId, start, end, "Duplicate")
      ).status
    );
  }
}
