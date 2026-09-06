Below is a complete implementation of `firstSuccessful` using `kotlinx.coroutines`. It runs all tasks concurrently, returns the first successful result, cancels unfinished tasks, propagates failures correctly, and handles all specified edge cases.

```kotlin
import kotlinx.coroutines.*
import kotlin.coroutines.cancellation.CancellationException

/**
 * Runs all [tasks] concurrently and returns the first successful result.
 *
 * - A failed task does not prevent other tasks from succeeding.
 * - After a success is obtained, all unfinished tasks are cancelled.
 * - If the list is empty, throws [IllegalArgumentException].
 * - If every task fails, throws [IllegalStateException] with all original
 *   exceptions attached as suppressed.
 * - Parent cancellation propagates normally and cancels all child tasks.
 */
suspend fun <T> firstSuccessful(tasks: List<suspend () -> T>): T = coroutineScope {
    require(tasks.isNotEmpty()) { "tasks cannot be empty" }

    val successDeferred = CompletableDeferred<T>()
    val failures = mutableListOf<Throwable>()

    // Launch each task in its own coroutine.
    val jobs = tasks.map { task ->
        launch {
            try {
                val result = task()
                successDeferred.complete(result)
            } catch (t: Throwable) {
                if (t is CancellationException) {
                    // Propagate cancellation, do not treat as a failure.
                    throw t
                } else {
                    synchronized(failures) {
                        failures.add(t)
                    }
                }
            }
        }
    }

    // A coroutine that throws if all tasks fail.
    val allFinished = launch {
        jobs.joinAll()
        if (!successDeferred.isCompleted) {
            val exception = IllegalStateException("All tasks failed")
            synchronized(failures) {
                failures.forEach { exception.addSuppressed(it) }
            }
            throw exception
        }
    }

    try {
        // Wait for the first success.
        val result = successDeferred.await()

        // Cancel all unfinished tasks and the failure-watcher.
        jobs.forEach { it.cancel() }
        allFinished.cancel()

        result
    } catch (e: CancellationException) {
        // Parent cancellation: ensure children are cancelled, then rethrow.
        jobs.forEach { it.cancel() }
        allFinished.cancel()
        throw e
    }
}
```

---

## Explanation of Cancellation Behaviour

- **Successful completion**
  When a task succeeds, its result is used to complete `successDeferred`.
  The awaiting coroutine resumes, cancels all launched jobs (the remaining tasks and the `allFinished` watcher), and returns the result.
  Cancellation is cooperative; tasks that are in a suspending call (e.g., `delay`, `awaitCancellation`) will throw `CancellationException` and finish.

- **Task failure**
  Failures are caught inside each task coroutine and stored in a synchronized list.
  They are **not** rethrown, so a single failure does not cancel the sibling tasks or the parent scope.
  The failure is recorded, and the coroutine completes normally.

- **All tasks fail**
  The separate `allFinished` coroutine waits for all task coroutines to complete.
  If `successDeferred` was never completed (i.e., no success happened), it creates an `IllegalStateException`, attaches all recorded failures as suppressed exceptions, and throws it.
  Because the exception is thrown from a child coroutine of the `coroutineScope`, the scope cancels any remaining children (none are left) and rethrows the `IllegalStateException` to the caller.

- **Parent cancellation**
  If the caller’s coroutine is cancelled, the `coroutineScope`’s job is automatically cancelled, which cancels all launched children (the task coroutines and `allFinished`).
  Inside a task, cancellation appears as a `CancellationException`; it is rethrown (not stored as a failure).
  In the main body, `successDeferred.await()` throws `CancellationException` because the calling coroutine is cancelled.
  The `catch` block cancels the jobs explicitly (already cancelled, but to be safe) and rethrows the exception.

---

## Unit Tests

The tests use `kotlinx.coroutines.test` to control coroutine execution deterministically.

```kotlin
import kotlinx.coroutines.*
import kotlinx.coroutines.test.runTest
import kotlin.test.*

class FirstSuccessfulTest {

    @Test
    fun `empty list throws IllegalArgumentException`() = runTest {
        assertFailsWith<IllegalArgumentException> {
            firstSuccessful<Int>(emptyList())
        }
    }

    @Test
    fun `all tasks fail throws IllegalStateException with suppressed exceptions`() = runTest {
        val ex1 = RuntimeException("ex1")
        val ex2 = IllegalArgumentException("ex2")

        val tasks = listOf<suspend () -> Int>(
            { throw ex1 },
            { throw ex2 }
        )

        val exception = assertFailsWith<IllegalStateException> {
            firstSuccessful(tasks)
        }

        assertEquals(2, exception.suppressed.size)
        assertTrue(exception.suppressed.contains(ex1))
        assertTrue(exception.suppressed.contains(ex2))
    }

    @Test
    fun `returns first successful result and cancels unfinished tasks`() = runTest {
        val firstStarted = CompletableDeferred<Unit>()
        val firstCancelled = CompletableDeferred<Unit>()

        val firstTask: suspend () -> Int = {
            firstStarted.complete(Unit)
            try {
                awaitCancellation()
            } catch (e: CancellationException) {
                firstCancelled.complete(Unit)
                throw e
            }
        }

        val secondTask: suspend () -> Int = { 42 }

        val result = firstSuccessful(listOf(firstTask, secondTask))

        assertEquals(42, result)
        firstStarted.await()       // First task did start
        firstCancelled.await()     // First task was cancelled after success
    }

    @Test
    fun `failed task does not prevent another from succeeding`() = runTest {
        val tasks = listOf<suspend () -> Int>(
            { throw RuntimeException("fail") },
            { 7 }
        )

        val result = firstSuccessful(tasks)

        assertEquals(7, result)
    }

    @Test
    fun `parent cancellation propagates and cancels all children`() = runTest {
        val taskStarted = CompletableDeferred<Unit>()
        val taskCancelled = CompletableDeferred<Unit>()

        val task: suspend () -> Int = {
            taskStarted.complete(Unit)
            try {
                awaitCancellation()
            } catch (e: CancellationException) {
                taskCancelled.complete(Unit)
                throw e
            }
        }

        val job = launch {
            firstSuccessful(listOf(task))
        }

        taskStarted.await()
        job.cancelAndJoin()

        assertTrue(job.isCancelled)
        taskCancelled.await()  // Confirm the underlying task was cancelled
    }
}
```

The implementation is fully non‑blocking, uses structured concurrency, and respects all requirements.
