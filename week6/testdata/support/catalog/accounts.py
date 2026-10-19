"""Controlled fixture: lookup does not normalize user input."""

ACCOUNTS = {"alex@example.test": "Alex"}


def find_account(email):
    return ACCOUNTS.get(email)
