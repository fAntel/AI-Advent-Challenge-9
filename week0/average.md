Here's the implementation of the `firstSuccessful` function with all the requirements:

```kotlin
import kotlinx.coroutines.*
import kotlinx.coroutines.selects.select

suspend fun <T> firstSuccessful(
    tasks: List<suspend () -> T>
): T {
    require(tasks.isNotEmpty()) { "Task list cannot be empty" }

    return coroutineScope {
        val deferreds = tasks.map { task ->
            async { task() }
        }

        val failures = mutableListOf<Throwable>()
        var result: T? = null

        try {
            // Wait for the first successful result
            result = select {
                deferreds.forEach { deferred ->
                    deferred.onAwait { value ->
                        value
                    }
                }
            }
        } catch (e: CancellationException) {
            // Parent cancellation - propagate
            throw e
        } catch (e: Throwable) {
            // A task failed - record it and try again
            failures.add(e)

            // Wait for remaining tasks
            var remainingDeferreds = deferreds.filter { !it.isCancelled && !it.isCompleted }

            while (remainingDeferreds.isNotEmpty() && result == null) {
                try {
                    result = select {
                        remainingDeferreds.forEach { deferred ->
                            deferred.onAwait { value ->
                                value
                            }
                        }
                    }
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Throwable) {
                    failures.add(e)
                    remainingDeferreds = deferreds.filter { !it.isCancelled && !it.isCompleted }
                }
            }

            if (result == null) {
                // All tasks failed
                val exception = IllegalStateException("All tasks failed")
                failures.forEach { failure ->
                    exception.addSuppressed(failure)
                }
                throw exception
            }
        }

        // Cancel all remaining tasks
        coroutineContext.cancelChildren()

        @Suppress("UNCHECKED_CAST")
        result as T
    }
}
```

Wait, there's a simpler and cleaner approach using `select` more effectively:

```kotlin
import kotlinx.coroutines.*
import kotlinx.coroutines.selects.select

suspend fun <T> firstSuccessful(
    tasks: List<suspend () -> T>
): T {
    require(tasks.isNotEmpty()) { "Task list cannot be empty" }

    return coroutineScope {
        val deferreds = tasks.map { task ->
            async {
                try {
                    Result.success(task())
                } catch (e: CancellationException) {
                    throw e
                } catch (e: Throwable) {
                    Result.failure<T>(e)
                }
            }
        }

        var failures = mutableListOf<Throwable>()

        // Wait for first successful result
        val result = try {
            select<T> {
                deferreds.forEach { deferred ->
                    deferred.onAwait { taskResult ->
                        taskResult.getOrThrow()
                    }
                }
            }
        } catch (e: CancellationException) {
            throw e
        } catch (e: Throwable) {
            // First task failed, collect failures from remaining tasks
            failures.add(e)

            val remainingDeferreds = deferreds.filter { !it.isCompleted }

            if (remainingDeferreds.isEmpty()) {
                val exception = IllegalStateException("All tasks failed")
                failures.forEach { failure ->
                    exception.addSuppressed(failure)
                }
                throw exception
            }

            var successfulResult: T? = null

            try {
                successfulResult = select<T> {
                    remainingDeferreds.forEach { deferred ->
                        deferred.onAwait { taskResult ->
                            taskResult.getOrThrow()
                        }
                    }
                }
            } catch (ce: CancellationException) {
                throw ce
            } catch (e2: Throwable) {
                failures.add(e2)
                val exception = IllegalStateException("All tasks failed")
                failures.forEach { failure ->
                    exception.addSuppressed(failure)
                }
                throw exception
            }

            successfulResult
        }

        // Cancel all remaining tasks
        deferreds.forEach { deferred ->
            if (!deferred.isCompleted) {
                deferred.cancel()
            }
        }

        result
    }
}
```

Actually, let me provide the cleanest and most correct solution:

```kotlin
import kotlinx.coroutines.*
import kotlinx.coroutines.selects.select

suspend fun <T> firstSuccessful(
    tasks: List<suspend () -> T>
): T {
    require(tasks.isNotEmpty()) { "Task list cannot be empty" }

    return coroutineScope {
        val deferredResults = tasks.map { task ->
            async {
                try {
                    Result.success(task())
                } catch (e: CancellationException) {
                    throw e  // Propagate cancellation
                } catch (e: Throwable) {
                    Result.failure<T>(e)  // Wrap non-cancellation failures
                }
            }
        }

        val failures = mutableListOf<Throwable>()
        var remainingDeferreds = deferredResults

        while (remainingDeferreds.isNotEmpty()) {
            try {
                // Wait for the first completed deferred
                val result = select<T> {
                    remainingDeferreds.forEach { deferred ->
                        deferred.onAwait { wrappedResult ->
                            wrappedResult.getOrThrow()  // Throw if failed, return if success
                        }
                    }
                }

                // We found a successful result!
                // Cancel all remaining tasks
                remainingDeferreds.forEach { it.cancel() }
                return@coroutineScope result

            } catch (e: CancellationException) {
                // Parent cancellation - propagate
                throw e
            } catch (e: Throwable) {
                // This task failed - record and continue with others
                failures.add(e)
                remainingDeferreds = remainingDeferreds.filter { !it.isCompleted }
            }
        }

        // All tasks failed
        val exception = IllegalStateException("All tasks failed")
        failures.forEach { failure ->
            exception.addSuppressed(failure)
        }
        throw exception
    }
}
```

Now, here are the unit tests:

```kotlin
import kotlinx.coroutines.*
import kotlinx.coroutines.test.*
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.Assertions.*
import kotlin.time.Duration.Companion.seconds

class FirstSuccessfulTest {

    @Test
    fun `returns first successful result regardless of order`() = runTest {
        val tasks = listOf(
            suspend {
                delay(1000)
                "slow"
            },
            suspend {
                delay(100)
                "fast"
            },
            suspend {
                delay(2000)
                "slowest"
            }
        )

        val result = firstSuccessful(tasks)
        assertEquals("fast", result)
    }

    @Test
    fun `failed task does not prevent success`() = runTest {
        val tasks = listOf(
            suspend {
                delay(100)
                throw RuntimeException("Fast failure")
            },
            suspend {
                delay(500)
                "success"
            }
        )

        val result = firstSuccessful(tasks)
        assertEquals("success", result)
    }

    @Test
    fun `throws IllegalArgumentException for empty list`() = runTest {
        assertThrows(IllegalArgumentException::class.java) {
            firstSuccessful(emptyList<suspend () -> String>())
        }
    }

    @Test
    fun `throws IllegalStateException with suppressed failures when all tasks fail`() = runTest {
        val tasks = listOf(
            suspend {
                delay(100)
                throw RuntimeException("Task 1 failed")
            },
            suspend {
                delay(200)
                throw IllegalArgumentException("Task 2 failed")
            },
            suspend {
                delay(150)
                throw IllegalStateException("Task 3 failed")
            }
        )

        val exception = assertThrows(IllegalStateException::class.java) {
            firstSuccessful(tasks)
        }

        assertEquals("All tasks failed", exception.message)
        assertEquals(3, exception.suppressed.size)
        assertTrue(exception.suppressed[0] is RuntimeException)
        assertTrue(exception.suppressed[1] is IllegalArgumentException)
        assertTrue(exception.suppressed[2] is IllegalStateException)
    }

    @Test
    fun `cancels unfinished tasks after success`() = runTest {
        var slowTaskCancelled = false
        var slowTaskCompleted = false

        val tasks = listOf(
            suspend {
                delay(100)
                "fast"
            },
            suspend {
                try {
                    delay(1000)
                    slowTaskCompleted = true
                    "slow"
                } catch (e: CancellationException) {
                    slowTaskCancelled = true
                    throw e
                }
            }
        )

        val result = firstSuccessful(tasks)

        assertEquals("fast", result)
        assertTrue(slowTaskCancelled)
        assertFalse(slowTaskCompleted)
    }

    @Test
    fun `parent cancellation propagates to children`() = runTest {
        var childWasCancelled = false

        val job = launch {
            firstSuccessful(listOf(
                suspend {
                    delay(100)
                    "fast"
                },
                suspend {
                    try {
                        delay(1000)
                        "slow"
                    } catch (e: CancellationException) {
                        childWasCancelled = true
                        throw e
                    }
                }
            ))
        }

        // Cancel parent
        delay(50)
        job.cancel()
        job.join()

        assertTrue(childWasCancelled)
    }

    @Test
    fun `handles concurrent successes correctly`() = runTest {
        val tasks = List(5) { index ->
            suspend {
                delay(100)
                "task-$index"
            }
        }

        val result = firstSuccessful(tasks)
        assertTrue(result.startsWith("task-"))
    }

    @Test
    fun `single task success`() = runTest {
        val tasks = listOf(
            suspend { "single" }
        )

        val result = firstSuccessful(tasks)
        assertEquals("single", result)
    }

    @Test
    fun `single task failure`() = runTest {
        val tasks = listOf(
            suspend {
                throw RuntimeException("Only task failed")
            }
        )

        val exception = assertThrows(IllegalStateException::class.java) {
            firstSuccessful(tasks)
        }

        assertEquals("All tasks failed", exception.message)
        assertEquals(1, exception.suppressed.size)
        assertTrue(exception.suppressed[0] is RuntimeException)
    }
}
```

## Explanation of Cancellation Behavior:

1. **Parent Cancellation**: The `coroutineScope` creates a new scope that is a child of the parent coroutine. If the parent is cancelled, the `coroutineScope` will throw a `CancellationException`, which we catch and rethrow to propagate the cancellation. All child coroutines (the `async` tasks) are automatically cancelled when the scope is cancelled.

2. **Successful Result Cancellation**: When we get a successful result, we explicitly cancel all remaining deferred tasks using `deferred.cancel()`. This immediately stops any ongoing work in those tasks.

3. **Task Failure Handling**: We wrap task execution in a `Result` object. This allows us to distinguish between a successful result and a failure without throwing the failure immediately. Failures are collected and only thrown as suppressed exceptions if all tasks fail.

4. **Cancellation Exceptions**: We treat `CancellationException` separately from other exceptions. Cancellation exceptions are always propagated immediately and are not treated as task failures, ensuring proper cancellation semantics.
