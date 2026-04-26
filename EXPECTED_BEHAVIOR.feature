Feature: Todo app expected behavior
  As a developer
  I want behavior written in executable-style scenarios
  So that system expectations are clear and testable

  Background:
    Given a todo store is initialized

  Scenario: Add creates a new todo with expected defaults
    When I add a todo with text "Write tests", tab "work", and deadline "2030-12-25"
    Then the created todo should have a non-empty ID
    And the created todo should have text "Write tests"
    And the created todo should have tab "work"
    And the created todo should have deadline "2030-12-25"
    And the created todo should be marked as not done
    And the created todo should have CreatedAt set to the current time

  Scenario: List returns only todos from the requested tab
    Given the store contains todos in tabs "work" and "private"
    When I list todos for tab "work"
    Then only todos with tab "work" should be returned

  Scenario: Toggle succeeds for existing todo
    Given a todo exists with ID "known-id" and done status false
    When I toggle todo "known-id"
    Then the toggle operation should return true
    And todo "known-id" should have done status true

  Scenario: Toggle fails for missing todo
    Given no todo exists with ID "missing-id"
    When I toggle todo "missing-id"
    Then the toggle operation should return false

  Scenario: Delete succeeds for existing todo
    Given a todo exists with ID "known-id"
    When I delete todo "known-id"
    Then the delete operation should return true
    And todo "known-id" should no longer exist in the store

  Scenario: Delete fails for missing todo
    Given no todo exists with ID "missing-id"
    When I delete todo "missing-id"
    Then the delete operation should return false

  Scenario: jsonResponse writes JSON with status and content type
    Given an HTTP response recorder
    When jsonResponse is called with status 201 and payload {"ok": true}
    Then the response status should be 201
    And the response Content-Type header should be "application/json"
    And the response body should be valid JSON
    And the response body field "ok" should be true

  Scenario: localIPv4 returns only IPv4-formatted addresses
    When I collect local IPv4 addresses
    Then each returned address should be in dotted IPv4 format "x.x.x.x"
