package dev.campusflow;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.security.config.Customizer;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.core.userdetails.*;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.csrf.CookieCsrfTokenRepository;
import org.springframework.security.web.csrf.CsrfTokenRequestAttributeHandler;

@Configuration
public class SecurityConfig {

  @Bean
  PasswordEncoder passwordEncoder() {
    return new BCryptPasswordEncoder();
  }

  @Bean
  UserDetailsService users(JdbcTemplate db) {
    return email ->
      db
        .query(
          "SELECT * FROM app_user WHERE email=?",
          (rs, n) ->
            User.withUsername(rs.getString("email"))
              .password(rs.getString("password_hash"))
              .roles(rs.getString("role"))
              .build(),
          email
        )
        .stream()
        .findFirst()
        .orElseThrow(() -> new UsernameNotFoundException("Unknown account"));
  }

  @Bean
  SecurityFilterChain filter(HttpSecurity http) throws Exception {
    return http
      .csrf(csrf ->
        csrf
          .csrfTokenRepository(CookieCsrfTokenRepository.withHttpOnlyFalse())
          .csrfTokenRequestHandler(new CsrfTokenRequestAttributeHandler())
      )
      .authorizeHttpRequests(auth ->
        auth
          .dispatcherTypeMatchers(jakarta.servlet.DispatcherType.ERROR)
          .permitAll()
          .requestMatchers("/api/csrf", "/actuator/health")
          .permitAll()
          .requestMatchers("/api/admin/**")
          .hasRole("ADMIN")
          .requestMatchers("/api/staff/**")
          .hasAnyRole("STAFF", "ADMIN")
          .anyRequest()
          .authenticated()
      )
      .httpBasic(Customizer.withDefaults())
      .sessionManagement(s ->
        s.sessionCreationPolicy(
          org.springframework.security.config.http.SessionCreationPolicy.STATELESS
        )
      )
      .build();
  }
}
